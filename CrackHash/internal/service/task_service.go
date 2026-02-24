package service

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/google/uuid"

	"CrackHash/internal/domain"
	"CrackHash/internal/metrics"
	"CrackHash/internal/repository"
)

type TaskService struct {
	repo repository.TaskRepository
}

func NewTaskService(repo repository.TaskRepository) *TaskService {
	return &TaskService{repo: repo}
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
) (*domain.Task, uint64, error) {

	signature := generateSignature(hash, algorithm, alphabet, maxLength)

	// Проверяем существующую
	existing, err := s.repo.GetBySignature(signature)
	if err == nil {
		return existing, estimateCombinations(alphabet, maxLength), nil
	}

	task := &domain.Task{
		ID:        uuid.New().String(),
		Hash:      hash,
		MaxLength: maxLength,
		Algorithm: algorithm,
		Alphabet:  alphabet,
		Signature: signature,
		Status:    domain.StatusInProgress,
		CreatedAt: time.Now(),
	}

	err = s.repo.Save(task)
	if err != nil {
		return nil, 0, err
	}

	// Пока симуляция
	go s.simulateExecution(task.ID)

	return task, estimateCombinations(alphabet, maxLength), nil
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
	var totalTime float64

	for _, t := range tasks {
		switch t.Status {
		case domain.StatusInProgress:
			active++
		case domain.StatusReady:
			completed++
			if t.FinishedAt != nil {
				totalTime += t.FinishedAt.Sub(t.CreatedAt).Seconds()
			}
		}
	}

	var avg float64
	if completed > 0 {
		avg = totalTime / float64(completed)
	}

	return metrics.Snapshot{
		TotalTasks:       total,
		ActiveTasks:      active,
		CompletedTasks:   completed,
		AvgExecutionTime: avg,
	}
}
