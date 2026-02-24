package repository

import (
	"errors"
	"sync"

	"CrackHash/internal/domain"
)

type MemoryRepository struct {
	mu             sync.RWMutex
	tasks          map[string]*domain.Task
	signatureIndex map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		tasks:          make(map[string]*domain.Task),
		signatureIndex: make(map[string]string),
	}
}

func (r *MemoryRepository) Save(task *domain.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks[task.ID] = task
	r.signatureIndex[task.Signature] = task.ID
	return nil
}

func (r *MemoryRepository) GetByID(id string) (*domain.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	task, ok := r.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (r *MemoryRepository) GetBySignature(signature string) (*domain.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.signatureIndex[signature]
	if !ok {
		return nil, errors.New("task not found")
	}

	return r.tasks[id], nil
}

func (r *MemoryRepository) Update(task *domain.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tasks[task.ID] = task
	return nil
}

func (r *MemoryRepository) List() ([]*domain.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		result = append(result, t)
	}
	return result, nil
}
