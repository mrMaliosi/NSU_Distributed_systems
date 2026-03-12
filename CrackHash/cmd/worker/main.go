package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"CrackHash/internal/api/http/route"
	"CrackHash/internal/service"
)

func main() {
	managerURL := os.Getenv("MANAGER_URL")
	if managerURL == "" {
		managerURL = "http://manager:57107"
	}

	// Таймаут на HTTP запросы (например, отправка результата менеджеру) и на таймауты HTTP-сервера воркера.
	workerTimeoutSec := 10
	if v := os.Getenv("WORKER_TIMEOUT_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			workerTimeoutSec = parsed
		}
	}
	timeout := time.Duration(workerTimeoutSec) * time.Second

	workerService := service.NewWorkerService(managerURL, timeout)
	route.RegisterWorkerRoutes(workerService)

	port := 57107
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	log.Printf("Worker running on port %d\n", port)
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           nil,
		ReadHeaderTimeout: timeout,
		ReadTimeout:       timeout,
		WriteTimeout:      timeout,
		IdleTimeout:       timeout,
	}
	log.Fatal(srv.ListenAndServe())
}
