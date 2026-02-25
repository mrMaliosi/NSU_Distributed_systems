package service

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
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
}

func NewTaskService(repo repository.TaskRepository, workerEndpoints []string) *TaskService {
	return &TaskService{
		repo:            repo,
		workerEndpoints: workerEndpoints,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
	}
}

func generateSignature(hash, algo, alphabet string, maxLen int) string {
	data := hash + "|" + algo + "|" + alphabet + "|" + strconv.Itoa(maxLen)
	sum := md5.Sum([]byte(data))
	return hex.EncodeToString(sum[:])
}

func estimateCombinations(alphabet string, maxLength int) uint64 {
	var total uint64
	base := uint64(len(alphabet))
	var pow uint64 = 1

	for i := 1; i <= maxLength; i++ {
		pow *= base
		total += pow
	}

	return total
}

func (s *TaskService) CreateTask(
	hash string,
	maxLength int,
	algorithm string,
	alphabet string,
) (*domain.Task, uint64, bool, error) {

	signature := generateSignature(hash, algorithm, alphabet, maxLength)
	estimated := estimateCombinations(alphabet, maxLength)

	// Проверяем, нет ли уже задачи с такими параметрами
	existing, err := s.repo.GetBySignature(signature)
	if err == nil && existing != nil {
		// Если задача уже есть и не была отменена — просто возвращаем её.
		if existing.Status != domain.StatusCancelled {
			return existing, estimated, true, nil
		}
	}

	splitter := NewSplitterService(alphabet, maxLength, 10*time.Second, 0)
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
		return nil, 0, false, err
	}

	go s.dispatchTaskParts(task, int(partCount))

	return task, estimated, false, nil
}

func (s *TaskService) dispatchTaskParts(task *domain.Task, partCount int) {
	if len(s.workerEndpoints) == 0 || partCount <= 0 {
		return
	}

	for partNumber := 0; partNumber < partCount; partNumber++ {
		workerURL := s.workerEndpoints[partNumber%len(s.workerEndpoints)]

		payload := dto.WorkerTaskRequest{
			RequestId:  task.ID,
			Hash:       task.Hash,
			PartNumber: partNumber,
			PartCount:  partCount,
			Algorithm:  task.Algorithm,
			Alphabet:   task.Alphabet,
		}

		go s.sendWorkerTask(workerURL, payload)
	}
}

// sendWorkerTask отправляет одну часть задачи конкретному воркеру.
func (s *TaskService) sendWorkerTask(workerURL string, payload dto.WorkerTaskRequest) {
	if workerURL == "" {
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := workerURL + "/internal/api/worker/hash/crack/task"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (s *TaskService) simulateExecution(taskID string) {
	time.Sleep(5 * time.Second)

	task, err := s.repo.GetByID(taskID)
	if err != nil {
		return
	}

	task.Status = domain.StatusReady
	now := time.Now()
	task.FinishedAt = &now
	task.Result = []string{"abcd"}

	s.repo.Update(task)
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
			// Средняя скорость по задаче = checkedWords / (totalExecTimeSec)
			if t.TotalExecTimeMs > 0 && t.CheckedWords > 0 {
				seconds := float64(t.TotalExecTimeMs) / 1000.0
				if seconds > 0 {
					speed := float64(t.CheckedWords) / seconds
					totalSpeed += speed
					speedSamples++
				}
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

	task, err := s.repo.GetByID(requestID)
	if err != nil {
		return err
	}

	// Если задача уже завершена или отменена — игнорируем
	if task.Status == domain.StatusReady ||
		task.Status == domain.StatusCancelled {
		return nil
	}

	// Если воркер вернул ошибку
	if workerErr != "" {
		task.FailedParts = append(task.FailedParts, partNumber)
		// Переводим задачу в статус ошибки и сохраняем текст
		task.Status = domain.StatusError
		task.Error = workerErr
		now := time.Now()
		task.FinishedAt = &now
		return s.repo.Update(task)
	}

	// Добавляем найденные слова
	task.Result = append(task.Result, words...)

	// Обновляем метрики: сколько слов проверено и сколько времени заняло
	task.CheckedWords += checked
	if execTime > 0 {
		task.TotalExecTimeMs += uint64(execTime)
	}

	// Увеличиваем счётчик обработанных частей
	task.CompletedParts++

	// Если все части выполнены — завершаем задачу
	if task.CompletedParts >= task.TotalParts {
		task.Status = domain.StatusReady
		now := time.Now()
		task.FinishedAt = &now
	}

	return s.repo.Update(task)
}
