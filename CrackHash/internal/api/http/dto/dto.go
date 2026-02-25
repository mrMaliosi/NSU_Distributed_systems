package dto

import (
	"CrackHash/internal/domain"
)

type CrackResponse struct {
	RequestID             string `json:"requestId"`
	EstimatedCombinations uint64 `json:"estimatedCombinations"`
}
type CrackRequest struct {
	Hash      string `json:"hash"`
	MaxLength int    `json:"maxLength"`
	Algorithm string `json:"algorithm"`
	Alphabet  string `json:"alphabet"`
}

type StatusResponse struct {
	Status domain.Status `json:"status"`
	Data   []string      `json:"data"`
	Error  string        `json:"error,omitempty"`
}

type MetricsResponse struct {
	TotalTasks       int     `json:"totalTasks"`
	ActiveTasks      int     `json:"activeTasks"`
	CompletedTasks   int     `json:"completedTasks"`
	AvgExecutionTime float64 `json:"avgExecutionTime"`
}

type WorkerTaskRequest struct {
	RequestId  string `json:"requestId"`
	Hash       string `json:"hash"`
	PartNumber int    `json:"partNumber"`
	PartCount  int    `json:"partCount"`
	Algorithm  string `json:"algorithm"`
	Alphabet   string `json:"alphabet"`
}

type WorkerResultResponse struct {
	RequestId     string   `json:"requestId"`
	PartNumber    int      `json:"partNumber"`
	WordsList     []string `json:"wordsList"`
	WordsNum      uint64   `json:"wordsNum"`
	ExecutionTime int64    `json:"workingTime"`
	Error         string   `json:"error,omitempty"`
}
