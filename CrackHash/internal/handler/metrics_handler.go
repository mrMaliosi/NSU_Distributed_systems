package handler

import (
	"encoding/json"
	"net/http"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/metrics"
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

func (h *MetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snapshot := h.service.GetMetrics()

	resp := dto.MetricsResponse{
		TotalTasks:       snapshot.TotalTasks,
		ActiveTasks:      snapshot.ActiveTasks,
		CompletedTasks:   snapshot.CompletedTasks,
		AvgExecutionTime: snapshot.AvgExecutionTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
