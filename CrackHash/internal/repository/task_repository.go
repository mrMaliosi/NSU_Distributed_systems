package repository

type TaskRepository interface {
	Save(task *Task) error
	GetByID(id string) (*Task, error)
	GetBySignature(signature string) (*Task, error)
	Update(task *Task) error
	List() ([]*Task, error)
}
