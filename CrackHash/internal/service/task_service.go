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
	"time"

	"github.com/google/uuid"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/domain"
	"CrackHash/internal/metrics"
	"CrackHash/internal/repository"
	"CrackHash/internal/scheduler"
)

type TaskService struct {
	repo            repository.TaskRepository
	workerEndpoints []string
	httpClient      *http.Client
	timeout         time.Duration

	mu         sync.Mutex
	schedulers map[string]*scheduler.Scheduler
}

const maxWorkerRetriesPerPart = 5

func NewTaskService(repo repository.TaskRepository, workerEndpoints []string, timeout time.Duration) *TaskService {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	return &TaskService{
		repo:            repo,
		workerEndpoints: workerEndpoints,
		httpClient:      &http.Client{Timeout: timeout},
		timeout:         timeout,
		schedulers:      make(map[string]*scheduler.Scheduler),
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

	sch := scheduler.New(task.ID, partCount)

	s.mu.Lock()
	s.schedulers[task.ID] = sch
	s.mu.Unlock()

	send := func(workerURL string, partNumber int) bool {
		payload := dto.WorkerTaskRequest{
			RequestId:  task.ID,
			MaxLength:  task.MaxLength,
			Hash:       task.Hash,
			PartNumber: partNumber,
			PartCount:  partCount,
			Algorithm:  task.Algorithm,
			Alphabet:   task.Alphabet,
		}
		ok, _ := s.sendWorkerTask(workerURL, payload)
		return ok
	}

	shouldStop := func() bool {
		current, err := s.repo.GetByID(task.ID)
		if err != nil {
			return true
		}
		return current.Status == domain.StatusCancelled || current.Status == domain.StatusReady || current.Status == domain.StatusError
	}

	sch.Start(s.workerEndpoints, send, shouldStop)
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

		tries := sch.IncRetry(partNumber)
		if tries > maxWorkerRetriesPerPart {
			task.Status = domain.StatusError
			task.Error = workerErr
			now := time.Now()
			task.FinishedAt = &now
			return s.repo.Update(task)
		}

		_ = s.repo.Update(task)
		sch.EnqueueRetry(partNumber)
		return nil
	}

	// Идемпотентность по partNumber без map.
	// Если из-за сетевых сбоев/таймаута пришёл дубль результата — второй раз не засчитываем.
	if sch != nil {
		if !sch.MarkDone(partNumber) {
			return nil
		}
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
