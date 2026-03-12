package handler

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/service"
)

type WorkerHandler struct {
	workerService *service.WorkerService
	busy          atomic.Bool
}

func NewWorkerHandler(workerService *service.WorkerService) *WorkerHandler {
	return &WorkerHandler{workerService: workerService}
}

func (h *WorkerHandler) HandleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Разрешаем одновременно обрабатывать ровно одну подзадачу.
	// Если воркер занят — отвечаем сразу, чтобы менеджер переназначил partNumber другому воркеру.
	if !h.busy.CompareAndSwap(false, true) {
		http.Error(w, "worker is busy", http.StatusServiceUnavailable)
		return
	}

	var req dto.WorkerTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.busy.Store(false)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	go func() {
		defer h.busy.Store(false)
		h.workerService.ProcessTask(req)
	}()

	w.WriteHeader(http.StatusAccepted)
}
