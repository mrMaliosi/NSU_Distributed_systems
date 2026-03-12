package scheduler

import (
	"sync/atomic"
	"time"
)

// Sender вызывается планировщиком, чтобы "назначить" partNumber конкретному воркеру.
// Должен вернуть true только если воркер действительно принял задачу (например, HTTP 202).
type Sender func(workerURL string, partNumber int) bool

// Stopper возвращает true, если задачу больше не нужно планировать (READY/ERROR/CANCELLED и т.п.).
type Stopper func() bool

// Scheduler гарантирует, что каждый воркер (endpoint) имеет максимум 1 in-flight подзадачу,
// а при отказах подзадачи перекидываются на другие воркеры.
type Scheduler struct {
	taskID    string
	partCount int

	nextPart atomic.Int64

	retryQueue chan int
	retries    []atomic.Int32

	// done[partNumber] == 1 если часть уже успешно зачтена
	done      []atomic.Uint32
	doneCount atomic.Int64
}

func New(taskID string, partCount int) *Scheduler {
	if partCount < 0 {
		partCount = 0
	}

	s := &Scheduler{
		taskID:     taskID,
		partCount:  partCount,
		retryQueue: make(chan int, partCount*2),
		retries:    make([]atomic.Int32, partCount),
		done:       make([]atomic.Uint32, partCount),
	}
	s.nextPart.Store(0)
	return s
}

func (s *Scheduler) TaskID() string { return s.taskID }

func (s *Scheduler) Start(workerURLs []string, send Sender, shouldStop Stopper) {
	if s.partCount <= 0 || len(workerURLs) == 0 || send == nil || shouldStop == nil {
		return
	}
	for _, workerURL := range workerURLs {
		workerURL := workerURL
		go s.workerLoop(workerURL, send, shouldStop)
	}
}

func (s *Scheduler) workerLoop(workerURL string, send Sender, shouldStop Stopper) {
	backoff := 100 * time.Millisecond

	for {
		if shouldStop() || s.doneCount.Load() >= int64(s.partCount) {
			return
		}

		partNumber, ok := s.nextPartNumber()
		if !ok {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		if send(workerURL, partNumber) {
			backoff = 100 * time.Millisecond
			continue
		}

		// Busy/timeout/недоступен/не принял: возвращаем partNumber в очередь ретраев.
		s.EnqueueRetry(partNumber)
		time.Sleep(backoff)
		if backoff < time.Second {
			backoff *= 2
			if backoff > time.Second {
				backoff = time.Second
			}
		}
	}
}

func (s *Scheduler) EnqueueRetry(partNumber int) {
	if partNumber < 0 || partNumber >= s.partCount {
		return
	}
	if s.done[partNumber].Load() == 1 {
		return
	}
	select {
	case s.retryQueue <- partNumber:
	default:
	}
}

// MarkDone помечает часть как выполненную. Возвращает true только при первом вызове для partNumber.
func (s *Scheduler) MarkDone(partNumber int) bool {
	if partNumber < 0 || partNumber >= s.partCount {
		return false
	}
	if !s.done[partNumber].CompareAndSwap(0, 1) {
		return false
	}
	s.doneCount.Add(1)
	return true
}

// IncRetry увеличивает счётчик попыток для partNumber и возвращает текущее число попыток.
func (s *Scheduler) IncRetry(partNumber int) int {
	if partNumber < 0 || partNumber >= s.partCount {
		return 0
	}
	return int(s.retries[partNumber].Add(1))
}

func (s *Scheduler) nextPartNumber() (int, bool) {
	// Сначала ретраи
	select {
	case p := <-s.retryQueue:
		if p >= 0 && p < s.partCount && s.done[p].Load() == 0 {
			return p, true
		}
	default:
	}

	// Затем новый partNumber
	p := int(s.nextPart.Add(1) - 1)
	if p < 0 || p >= s.partCount {
		return 0, false
	}
	if s.done[p].Load() == 1 {
		return 0, false
	}
	return p, true
}
