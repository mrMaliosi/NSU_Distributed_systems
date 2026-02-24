package http

type CrackResponse struct {
	RequestID             string `json:"requestId"`
	EstimatedCombinations uint64 `json:"estimatedCombinations"`
}

type StatusResponse struct {
	Status Status   `json:"status"`
	Data   []string `json:"data"`
	Error  string   `json:"error,omitempty"`
}

type MetricsResponse struct {
	TotalTasks       int     `json:"totalTasks"`
	ActiveTasks      int     `json:"activeTasks"`
	CompletedTasks   int     `json:"completedTasks"`
	AvgExecutionTime float64 `json:"avgExecutionTime"`
}
