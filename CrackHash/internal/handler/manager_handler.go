package handler

import (
	"encoding/json"
	"net/http"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/domain"
	"CrackHash/internal/service"
)

type ManagerHandler struct {
	taskService *service.TaskService
}

func NewManagerHandler(taskService *service.TaskService) *ManagerHandler {
	return &ManagerHandler{
		taskService: taskService,
	}
}

func (h *ManagerHandler) HandleCrack(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodPost:
		var req dto.CrackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		task, est, existed, err := h.taskService.CreateTask(
			req.Hash,
			req.MaxLength,
			req.Algorithm,
			req.Alphabet,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Если задача уже существовала и она в статусе READY — сразу возвращаем готовый результат
		if existed && task.Status == domain.StatusReady {
			resp := dto.StatusResponse{
				Status: task.Status,
				Data:   task.Result,
				Error:  task.Error,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// В остальных случаях (новая задача или IN_PROGRESS) — обычный ответ с requestId и estimatedCombinations

		resp := dto.CrackResponse{
			RequestID:             task.ID,
			EstimatedCombinations: est,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)

	case http.MethodDelete:
		requestID := r.URL.Query().Get("requestId")
		if requestID == "" {
			http.Error(w, "requestId required", http.StatusBadRequest)
			return
		}

		if err := h.taskService.CancelTask(requestID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ManagerHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	requestID := r.URL.Query().Get("requestId")
	if requestID == "" {
		http.Error(w, "requestId required", http.StatusBadRequest)
		return
	}

	task, err := h.taskService.GetStatus(requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var data []string
	if task.Status == domain.StatusReady {
		data = task.Result
	}

	resp := dto.StatusResponse{
		Status: task.Status,
		Data:   data,
		Error:  task.Error,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *ManagerHandler) HandleCrackResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req dto.WorkerResultResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.taskService.AcceptWorkerResult(
		req.RequestId,
		req.PartNumber,
		req.WordsList,
		req.WordsNum,
		req.ExecutionTime,
		req.Error,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
