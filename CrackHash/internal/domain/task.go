package domain

import "time"

type Task struct {
	ID        string
	Hash      string
	MaxLength int
	Algorithm string
	Alphabet  string

	Status Status
	Result []string
	Error  string

	CreatedAt  time.Time
	FinishedAt *time.Time
}
