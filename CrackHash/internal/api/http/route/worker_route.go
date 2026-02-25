package route

import (
	"net/http"

	"CrackHash/internal/handler"
	"CrackHash/internal/service"
)

func RegisterWorkerRoutes(workerService *service.WorkerService) {
	handler := handler.NewWorkerHandler(workerService)

	http.HandleFunc("/internal/api/worker/hash/crack/task", handler.HandleTask)
}
