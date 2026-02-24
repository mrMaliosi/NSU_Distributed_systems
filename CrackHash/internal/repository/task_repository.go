package repository

import "CrackHash/internal/domain"

type TaskRepository interface {
	Save(task *domain.Task) error
	GetByID(id string) (*domain.Task, error)
	GetBySignature(signature string) (*domain.Task, error)
	Update(task *domain.Task) error
	List() ([]*domain.Task, error)
}
