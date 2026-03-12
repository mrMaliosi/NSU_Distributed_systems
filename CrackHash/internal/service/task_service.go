package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/domain"
	"CrackHash/internal/metrics"
	"CrackHash/internal/repository"
)

type TaskService struct {
	repo            repository.TaskRepository
	workerEndpoints []string
	httpClient      *http.Client
	timeout         time.Duration

	mu         sync.Mutex
	schedulers map[string]*taskScheduler
}

const maxWorkerRetriesPerPart = 5

// taskScheduler реализует "синхронную" отправку: каждый воркер (endpoint) получает
// максимум 1 подзадачу одновременно. Новые partNumber берём из атомарного счётчика,
// а повторные — из очереди retryQueue.
type taskScheduler struct {
	taskID    string
	partCount int

	nextPart atomic.Int64

	retryQueue chan int
	retries    []atomic.Int32

	// done[partNumber] == 1 если часть уже успешно зачтена (идемпотентность без map)
	done      []atomic.Uint32
	doneCount atomic.Int64
}

func NewTaskService(repo repository.TaskRepository, workerEndpoints []string, timeout time.Duration) *TaskService {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &TaskService{
		repo:            repo,
		workerEndpoints: workerEndpoints,
		httpClient:      &http.Client{Timeout: timeout},
		timeout:         timeout,
		schedulers:      make(map[string]*taskScheduler),
	}
}

func generateSignature(hash, algo, alphabet string, maxLen int) string {
	data := hash + "|" + algo + "|" + alphabet + "|" + strconv.Itoa(maxLen)
	sum := md5.Sum([]byte(data))
	return hex.EncodeToString(sum[:])
}

func estimateCombinations(alphabet string, maxLength int) *big.Int {
	if maxLength < 1 || len(alphabet) == 0 {
		return big.NewInt(0)
	}

	base := big.NewInt(int64(len(alphabet)))
	pow := big.NewInt(1)
	total := big.NewInt(0)

	for i := 1; i <= maxLength; i++ {
		pow.Mul(pow, base)
		total.Add(total, pow)
	}

	return total
}

func (s *TaskService) CreateTask(
	hash string,
	maxLength int,
	algorithm string,
	alphabet string,
) (*domain.Task, *big.Int, bool, error) {

	estimated := estimateCombinations(alphabet, maxLength)
	signature := generateSignature(hash, algorithm, alphabet, maxLength)

	// Проверяем, нет ли уже задачи с такими параметрами
	// Если задача уже есть и не была отменена — просто возвращаем её.
	existing, err := s.repo.GetBySignature(signature)
	if err == nil && existing != nil {
		if existing.Status != domain.StatusCancelled {
			return existing, estimated, true, nil
		}
	}

	splitter := NewSplitterService(alphabet, maxLength, s.timeout, 0)
	partCount := splitter.PartCount()

	task := &domain.Task{
		ID:             uuid.New().String(),
		Hash:           hash,
		MaxLength:      maxLength,
		Algorithm:      algorithm,
		Alphabet:       alphabet,
		Signature:      signature,
		Status:         domain.StatusInProgress,
		Result:         []string{},
		TotalParts:     partCount,
		CompletedParts: 0,
		CreatedAt:      time.Now(),
	}

	err = s.repo.Save(task)
	if err != nil {
		return nil, big.NewInt(0), false, err
	}
	fmt.Println("Accepted task with partCount", partCount)
	go s.startScheduler(task, int(partCount))

	return task, estimated, false, nil
}

func (s *TaskService) startScheduler(task *domain.Task, partCount int) {
	if len(s.workerEndpoints) == 0 || partCount <= 0 {
		return
	}

	sch := &taskScheduler{
		taskID:     task.ID,
		partCount:  partCount,
		retryQueue: make(chan int, partCount*2),
		retries:    make([]atomic.Int32, partCount),
		done:       make([]atomic.Uint32, partCount),
	}
	sch.nextPart.Store(0)

	s.mu.Lock()
	s.schedulers[task.ID] = sch
	s.mu.Unlock()

	// Один цикл на endpoint: на стороне воркера очереди нет (воркер держит busy=true),
	// а менеджер не будет слать ему следующую подзадачу, пока предыдущая не "принята" (202).
	for _, workerURL := range s.workerEndpoints {
		workerURL := workerURL
		go s.workerLoop(task, sch, workerURL)
	}
}

func (s *TaskService) workerLoop(task *domain.Task, sch *taskScheduler, workerURL string) {
	backoff := 100 * time.Millisecond

	for {
		// stop conditions
		current, err := s.repo.GetByID(sch.taskID)
		if err != nil {
			return
		}
		if current.Status == domain.StatusCancelled || current.Status == domain.StatusReady || current.Status == domain.StatusError {
			return
		}
		if sch.doneCount.Load() >= int64(sch.partCount) {
			return
		}

		partNumber, ok := sch.nextPartNumber()
		if !ok {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		payload := dto.WorkerTaskRequest{
			RequestId:  task.ID,
			MaxLength:  task.MaxLength,
			Hash:       task.Hash,
			PartNumber: partNumber,
			PartCount:  sch.partCount,
			Algorithm:  task.Algorithm,
			Alphabet:   task.Alphabet,
		}

		accepted, _ := s.sendWorkerTask(workerURL, payload)
		if accepted {
			backoff = 100 * time.Millisecond
			continue
		}

		// Busy/timeout/недоступен/не 202: возвращаем partNumber в очередь ретраев
		sch.enqueueRetry(partNumber)
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
			if backoff > time.Second {
				backoff = time.Second
			}
		}
	}
}

// sendWorkerTask отправляет одну часть задачи конкретному воркеру.
// Возвращает true только если воркер принял задачу (HTTP 202).
func (s *TaskService) sendWorkerTask(workerURL string, payload dto.WorkerTaskRequest) (bool, error) {
	if workerURL == "" {
		return false, fmt.Errorf("empty worker url")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	url := workerURL + "/internal/api/worker/hash/crack/task"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	resp.Body.Close()

	// 202 = Accepted (воркер взял работу). Всё остальное — считаем неуспехом.
	if resp.StatusCode == http.StatusAccepted {
		return true, nil
	}
	return false, fmt.Errorf("worker returned status %d", resp.StatusCode)
}

func (sch *taskScheduler) enqueueRetry(partNumber int) {
	if partNumber < 0 || partNumber >= sch.partCount {
		return
	}
	if sch.done[partNumber].Load() == 1 {
		return
	}
	select {
	case sch.retryQueue <- partNumber:
	default:
	}
}

func (sch *taskScheduler) nextPartNumber() (int, bool) {
	// Сначала ретраи
	select {
	case p := <-sch.retryQueue:
		if p >= 0 && p < sch.partCount && sch.done[p].Load() == 0 {
			return p, true
		}
	default:
	}

	// Затем новый partNumber
	p := int(sch.nextPart.Add(1) - 1)
	if p < 0 || p >= sch.partCount {
		return 0, false
	}
	if sch.done[p].Load() == 1 {
		return 0, false
	}
	return p, true
}

func (s *TaskService) GetStatus(id string) (*domain.Task, error) {
	return s.repo.GetByID(id)
}

func (s *TaskService) CancelTask(id string) error {
	task, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}

	task.Status = domain.StatusCancelled
	return s.repo.Update(task)
}

func (s *TaskService) GetMetrics() metrics.Snapshot {
	tasks, _ := s.repo.List()

	total := len(tasks)
	active := 0
	completed := 0
	var totalSpeed float64
	speedSamples := 0

	for _, t := range tasks {
		switch t.Status {
		case domain.StatusInProgress:
			active++
		case domain.StatusReady:
			completed++
		}

		// Средняя "скорость" задачи (слов в секунду) считаем для всех задач,
		// у которых уже есть какие‑то измерения, независимо от статуса.
		if t.TotalExecTimeMs > 0 && t.CheckedWords > 0 {
			seconds := float64(t.TotalExecTimeMs) / 1000.0
			if seconds > 0 {
				speed := float64(t.CheckedWords) / seconds
				totalSpeed += speed
				speedSamples++
			}
		}
	}

	var avg float64
	if speedSamples > 0 {
		avg = totalSpeed / float64(speedSamples)
	}

	return metrics.Snapshot{
		TotalTasks:       total,
		ActiveTasks:      active,
		CompletedTasks:   completed,
		AvgExecutionTime: avg,
	}
}

func (s *TaskService) AcceptWorkerResult(
	requestID string,
	partNumber int,
	words []string,
	checked uint64,
	execTime int64,
	workerErr string,
) error {
	fmt.Println(partNumber, checked, execTime)
	task, err := s.repo.GetByID(requestID)
	if err != nil {
		return err
	}

	// Если задача уже завершена или отменена — игнорируем
	if task.Status == domain.StatusReady ||
		task.Status == domain.StatusCancelled {
		return nil
	}

	s.mu.Lock()
	sch := s.schedulers[requestID]
	s.mu.Unlock()

	// Если воркер вернул ошибку
	if workerErr != "" {
		task.FailedParts = append(task.FailedParts, partNumber)

		// Повторно назначаем тот же partNumber: кладём в очередь ретраев.
		if sch == nil || partNumber < 0 || partNumber >= int(task.TotalParts) {
			task.Status = domain.StatusError
			task.Error = workerErr
			now := time.Now()
			task.FinishedAt = &now
			return s.repo.Update(task)
		}

		tries := sch.retries[partNumber].Add(1)
		if int(tries) > maxWorkerRetriesPerPart {
			task.Status = domain.StatusError
			task.Error = workerErr
			now := time.Now()
			task.FinishedAt = &now
			return s.repo.Update(task)
		}

		_ = s.repo.Update(task)
		sch.enqueueRetry(partNumber)
		return nil
	}

	// Идемпотентность по partNumber без map.
	// Если из-за сетевых сбоев/таймаута пришёл дубль результата — второй раз не засчитываем.
	if sch != nil && partNumber >= 0 && partNumber < sch.partCount {
		if !sch.done[partNumber].CompareAndSwap(0, 1) {
			return nil
		}
		sch.doneCount.Add(1)
	}

	// Добавляем найденные слова
	task.Result = append(task.Result, words...)

	// Обновляем метрики: сколько слов проверено и сколько времени заняло
	task.CheckedWords += checked
	if execTime > 0 {
		task.TotalExecTimeMs += uint64(execTime)
	}

	task.CompletedParts++
	if task.CompletedParts >= task.TotalParts {
		task.Status = domain.StatusReady
		now := time.Now()
		task.FinishedAt = &now
	}

	return s.repo.Update(task)
}
