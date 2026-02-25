package service

import (
	"math/big"
	"sync"
	"time"
)

type SplitterService struct {
	alphabet  string
	maxLength int
	timeout   time.Duration
	hashRate  float64 // комбинаций в секунду

	partCount uint64
}

var (
	instance *SplitterService
	once     sync.Once
)

//
// -------------------- Singleton Constructor --------------------
//

func NewSplitterService(
	alphabet string,
	maxLength int,
	timeout time.Duration,
	hashRate float64,
) *SplitterService {

	once.Do(func() {

		if hashRate <= 0 {
			hashRate = 1_000_000 // fallback 1M/s
		}

		s := &SplitterService{
			alphabet:  alphabet,
			maxLength: maxLength,
			timeout:   timeout,
			hashRate:  hashRate,
		}

		s.partCount = s.calculatePartCount()
		instance = s
	})

	return instance
}

//
// -------------------- Public API --------------------
//

// Возвращает количество частей
func (s *SplitterService) PartCount() uint64 {
	return s.partCount
}

// Вычисляет диапазон для конкретной части
func (s *SplitterService) ComputeRange(partNumber int) (*big.Int, *big.Int) {

	alphabetSize := int64(len(s.alphabet))

	// N = alphabetSize^maxLength
	N := bigPow(alphabetSize, int64(s.maxLength))

	partNum := big.NewInt(int64(partNumber))
	partCnt := big.NewInt(int64(s.partCount))

	// start = N * partNumber / partCount
	start := new(big.Int).Mul(N, partNum)
	start.Div(start, partCnt)

	// end = N * (partNumber+1) / partCount
	next := big.NewInt(int64(partNumber + 1))
	end := new(big.Int).Mul(N, next)
	end.Div(end, partCnt)

	return start, end
}

// Преобразование индекса в строку
func (s *SplitterService) IndexToWord(index *big.Int) string {

	base := big.NewInt(int64(len(s.alphabet)))
	zero := big.NewInt(0)

	result := make([]byte, s.maxLength)
	temp := new(big.Int).Set(index)

	for i := s.maxLength - 1; i >= 0; i-- {
		if temp.Cmp(zero) == 0 {
			result[i] = s.alphabet[0]
			continue
		}

		mod := new(big.Int)
		temp.DivMod(temp, base, mod)
		result[i] = s.alphabet[mod.Int64()]
	}

	return string(result)
}

//
// -------------------- Internal Logic --------------------
//

// Рассчёт количества частей
func (s *SplitterService) calculatePartCount() uint64 {

	alphabetSize := int64(len(s.alphabet))
	N := bigPow(alphabetSize, int64(s.maxLength))

	targetChunkSize := int64(s.hashRate * s.timeout.Seconds())
	if targetChunkSize <= 0 {
		targetChunkSize = 1
	}

	chunkSize := big.NewInt(targetChunkSize)

	partCount := new(big.Int).Div(N, chunkSize)

	mod := new(big.Int).Mod(N, chunkSize)
	if mod.Sign() != 0 {
		partCount.Add(partCount, big.NewInt(1))
	}

	// Ограничения безопасности
	maxParts := big.NewInt(1_000_000_000)
	if partCount.Cmp(maxParts) > 0 {
		partCount = maxParts
	}

	if partCount.Sign() == 0 {
		partCount = big.NewInt(1)
	}

	return partCount.Uint64()
}

//
// -------------------- Helpers --------------------
//

func bigPow(base int64, exp int64) *big.Int {
	b := big.NewInt(base)
	e := big.NewInt(exp)
	return new(big.Int).Exp(b, e, nil)
}
