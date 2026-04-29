package route

import (
	"net/http"

	"CrackHash/internal/handler"
	"CrackHash/internal/service"
)

func RegisterManagerRoutes(taskService *service.TaskService, metricsHandler http.Handler) {
	handler := handler.NewManagerHandler(taskService)

	http.HandleFunc("/api/hash/crack", handler.HandleCrack)
	http.HandleFunc("/api/hash/status", handler.HandleStatus)
	http.HandleFunc("/internal/api/manager/hash/crack/request", handler.HandleCrackResponse)

	http.Handle("/api/metrics", metricsHandler)
}
