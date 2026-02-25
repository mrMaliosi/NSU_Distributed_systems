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
	Status         Status
	Result         []string
	TotalParts     uint64
	CompletedParts uint64
	FailedParts    []int

	CreatedAt  time.Time
	FinishedAt *time.Time

	Error string
}
