package main

import (
	"log"
	"net/http"
	"strconv"

	"CrackHash/internal/api/http/route"
	"CrackHash/internal/handler"
	"CrackHash/internal/repository"
	"CrackHash/internal/service"
)

func main() {
	repo := repository.NewMemoryRepository()
	taskService := service.NewTaskService(repo)
	metricsHandler := handler.NewMetricsHandler(taskService)

	route.RegisterRoutes(taskService, metricsHandler)

	// Запуск сервера
	port := 57107
	log.Printf("Manager running on port %d\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
