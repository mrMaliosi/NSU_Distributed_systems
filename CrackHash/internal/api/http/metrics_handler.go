package http

import (
	"encoding/json"
	"net/http"
)

type MetricsProvider interface {
	GetMetrics() metrics.Snapshot
}

type MetricsHandler struct {
	service MetricsProvider
}

func NewMetricsHandler(service MetricsProvider) *MetricsHandler {
	return &MetricsHandler{service: service}
}

func (h *MetricsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	snapshot := h.service.GetMetrics()

	resp := MetricsResponse{
		TotalTasks:       snapshot.TotalTasks,
		ActiveTasks:      snapshot.ActiveTasks,
		CompletedTasks:   snapshot.CompletedTasks,
		AvgExecutionTime: snapshot.AvgExecutionTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
