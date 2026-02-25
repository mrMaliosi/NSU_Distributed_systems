package worker

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"CrackHash/internal/api/http/route"
	"CrackHash/internal/service"
)

func main() {
	// URL менеджера можно задать через MANAGER_URL, по умолчанию — имя сервиса из docker-compose
	managerURL := os.Getenv("MANAGER_URL")
	if managerURL == "" {
		managerURL = "http://manager:57107"
	}

	workerService := service.NewWorkerService(managerURL)
	route.RegisterWorkerRoutes(workerService)

	port := 57107
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	log.Printf("Worker running on port %d\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), nil))
}
