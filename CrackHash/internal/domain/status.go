package domain

type Status string

const (
	StatusInProgress Status = "IN_PROGRESS"
	StatusReady      Status = "READY"
	StatusError      Status = "ERROR"
	StatusCancelled  Status = "CANCELLED"
)
