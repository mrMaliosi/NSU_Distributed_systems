package worker

import (
	"crypto/md5"
	"encoding/hex"
)

func pow(base int, exp int) int {
	result := 1
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

func indexToWord(index int, alphabet string, length int) string {
	base := len(alphabet)
	word := make([]byte, length)

	for i := length - 1; i >= 0; i-- {
		remainder := index % base
		word[i] = alphabet[remainder]
		index /= base
	}

	return string(word)
}

func Process(hash, algorithm, alphabet string, partNumber, partCount int) ([]string, uint64, error) {

	var found []string
	var checked uint64

	// пример: перебор фиксированной длины 4
	length := 4

	total := pow(len(alphabet), length)
	chunk := total / partCount

	start := partNumber * chunk
	end := start + chunk

	for i := start; i < end; i++ {
		word := indexToWord(i, alphabet, length)
		checked++

		if checkHash(word, hash, algorithm) {
			found = append(found, word)
		}
	}

	return found, checked, nil
}

func checkHash(word, target, algorithm string) bool {
	switch algorithm {
	case "MD5":
		h := md5.Sum([]byte(word))
		return hex.EncodeToString(h[:]) == target
	}
	return false
}
