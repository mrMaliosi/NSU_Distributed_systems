package domain

import "time"

type Task struct {
	ID string

	// Параметры задачи
	Hash      string
	MaxLength int
	Algorithm string
	Alphabet  string

	Signature string // уникальная подпись задачи по её параметрам

	// Статус и результаты
	Status Status
	Result []string
	Error  string

	CreatedAt  time.Time
	FinishedAt *time.Time
}
