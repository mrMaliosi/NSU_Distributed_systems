package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"CrackHash/internal/api/http/dto"
	"CrackHash/internal/worker"
)

type WorkerService struct {
	managerURL string
	httpClient *http.Client
	maxRetries int
}

func NewWorkerService(managerURL string) *WorkerService {
	return &WorkerService{
		managerURL: managerURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		maxRetries: 5,
	}
}

func (s *WorkerService) ProcessTask(req dto.WorkerTaskRequest) {
	start := time.Now()

	words, checked, err := worker.Process(
		req.Hash,
		req.Algorithm,
		req.Alphabet,
		req.PartNumber,
		req.PartCount,
	)

	result := dto.WorkerResultResponse{
		RequestId:     req.RequestId,
		PartNumber:    req.PartNumber,
		WordsList:     words,
		WordsNum:      checked,
		ExecutionTime: time.Since(start).Milliseconds(),
	}

	if err != nil {
		result.Error = err.Error()
	}

	s.sendResultWithRetry(result)
}

func (s *WorkerService) sendResultWithRetry(result dto.WorkerResultResponse) {
	body, _ := json.Marshal(result)

	url := s.managerURL + "/internal/api/manager/hash/crack/request"

	for i := 0; i < s.maxRetries; i++ {
		req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(body))
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return
		}

		time.Sleep(2 * time.Second)
	}
}
