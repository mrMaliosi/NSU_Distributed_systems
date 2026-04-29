package worker

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
	"testing"
)

// Проверяем корректность вычисления и сравнения хэша
// для слов разных длин.
func TestCheckHash_VariousLengths(t *testing.T) {
	lengths := []int{1, 4, 8, 9, 10, 16, 64, 128}

	for _, l := range lengths {
		l := l
		t.Run(
			// имя саб‑теста вида "len=4"
			"len="+strconv.Itoa(l),
			func(t *testing.T) {
				word := make([]byte, l)
				for i := range word {
					word[i] = 'z'
				}

				sum := md5.Sum(word)
				hash := hex.EncodeToString(sum[:])

				if !checkHash(string(word), hash, "MD5") {
					t.Fatalf("expected checkHash to return true for length=%d", l)
				}

				// Проверяем, что на неправильный хэш функция отвечает false.
				badHash := "ffffffffffffffffffffffffffffffff"
				if checkHash(string(word), badHash, "MD5") {
					t.Fatalf("expected checkHash to return false for bad hash, length=%d", l)
				}
			},
		)
	}
}

// Проверяем, что Process действительно находит слово среди всех длин ≤ maxLength
// при одном воркере (partCount=1).
func TestProcess_FindsWord_SinglePart(t *testing.T) {
	alphabet := "abc"
	maxLength := 3

	target := "ba"
	sum := md5.Sum([]byte(target))
	hash := hex.EncodeToString(sum[:])

	words, checked, err := Process(hash, "MD5", alphabet, maxLength, 0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checked == 0 {
		t.Fatalf("expected checked > 0")
	}

	found := false
	for _, w := range words {
		if w == target {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find %q in %v", target, words)
	}
}

// Проверяем корректность разбиения пространства между несколькими частями:
// при суммировании checked по всем частям получаем общее количество комбинаций.
func TestProcess_MultiPart_Coverage(t *testing.T) {
	alphabet := "ab"
	maxLength := 3
	partCount := 3

	// Общее количество комбинаций для длин 1..maxLength:
	// 2^1 + 2^2 + 2^3 = 14
	const expectedTotal = 14

	var totalChecked uint64
	target := "ab"
	sum := md5.Sum([]byte(target))
	hash := hex.EncodeToString(sum[:])

	found := false

	for partNumber := 0; partNumber < partCount; partNumber++ {
		words, checked, err := Process(hash, "MD5", alphabet, maxLength, partNumber, partCount)
		if err != nil {
			t.Fatalf("unexpected error for part %d: %v", partNumber, err)
		}
		totalChecked += checked

		for _, w := range words {
			if w == target {
				found = true
			}
		}
	}

	if totalChecked != expectedTotal {
		t.Fatalf("expected totalChecked=%d, got %d", expectedTotal, totalChecked)
	}
	if !found {
		t.Fatalf("expected to find %q across all parts", target)
	}
}
