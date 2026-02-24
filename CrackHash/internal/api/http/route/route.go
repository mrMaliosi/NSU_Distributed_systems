package route

import (
	"encoding/json"
	"net/http"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/service"
)

func RegisterRoutes(taskService *service.TaskService, metricsHandler http.Handler) {
	http.HandleFunc("/api/hash/crack", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var req dto.CrackRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			task, est, err := taskService.CreateTask(req.Hash, req.MaxLength, req.Algorithm, req.Alphabet)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

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
			if err := taskService.CancelTask(requestID); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/hash/status", func(w http.ResponseWriter, r *http.Request) {
		requestID := r.URL.Query().Get("requestId")
		if requestID == "" {
			http.Error(w, "requestId required", http.StatusBadRequest)
			return
		}

		task, err := taskService.GetStatus(requestID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		resp := dto.StatusResponse{
			Status: task.Status,
			Data:   task.Result,
			Error:  task.Error,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	// Метрики
	http.Handle("/api/metrics", metricsHandler)
}
