package handler

import (
	"encoding/json"
	"net/http"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/service"
)

type WorkerHandler struct {
	workerService *service.WorkerService
}

func NewWorkerHandler(workerService *service.WorkerService) *WorkerHandler {
	return &WorkerHandler{workerService: workerService}
}

func (h *WorkerHandler) HandleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.WorkerTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go h.workerService.ProcessTask(req)

	w.WriteHeader(http.StatusAccepted)
}
