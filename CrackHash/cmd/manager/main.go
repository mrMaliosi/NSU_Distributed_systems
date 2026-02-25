package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"CrackHash/internal/api/http/route"
	"CrackHash/internal/handler"
	"CrackHash/internal/repository"
	"CrackHash/internal/service"
)

func main() {
	repo := repository.NewMemoryRepository()

	workerURLsEnv := os.Getenv("WORKER_URLS")
	var workerURLs []string
	if workerURLsEnv != "" {
		for _, u := range strings.Split(workerURLsEnv, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				workerURLs = append(workerURLs, u)
			}
		}
	}

	if len(workerURLs) == 0 {
		if cntStr := os.Getenv("WORKERS_COUNT"); cntStr != "" {
			if cnt, err := strconv.Atoi(cntStr); err == nil && cnt > 0 {
				baseURL := "http://worker:57107"
				workerURLs = make([]string, cnt)
				for i := 0; i < cnt; i++ {
					workerURLs[i] = baseURL
				}
			}
		}
	}

	if len(workerURLs) == 0 {
		workerURLs = []string{"http://worker:57107"}
	}

	taskService := service.NewTaskService(repo, workerURLs)
	metricsHandler := handler.NewMetricsHandler(taskService)

	route.RegisterManagerRoutes(taskService, metricsHandler)

	// Запуск сервера
	port := 57107
	log.Printf("Manager running on port %d\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
