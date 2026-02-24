package metrics

type Snapshot struct {
	TotalTasks       int
	ActiveTasks      int
	CompletedTasks   int
	AvgExecutionTime float64
}
