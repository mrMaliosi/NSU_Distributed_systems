package worker

import (
	"crypto/md5"
	"encoding/hex"
	"math/big"
)

// bigPow вычисляет base^exp в виде big.Int.
func bigPow(base int64, exp int) *big.Int {
	b := big.NewInt(base)
	e := big.NewInt(int64(exp))
	return new(big.Int).Exp(b, e, nil)
}

// indexToWordBig преобразует big.Int‑индекс в слово заданной длины.
func indexToWordBig(index *big.Int, alphabet string, length int) string {
	base := big.NewInt(int64(len(alphabet)))
	zero := big.NewInt(0)

	word := make([]byte, length)
	tmp := new(big.Int).Set(index)

	for i := length - 1; i >= 0; i-- {
		if tmp.Cmp(zero) == 0 {
			word[i] = alphabet[0]
			continue
		}

		mod := new(big.Int)
		tmp.DivMod(tmp, base, mod)
		word[i] = alphabet[mod.Int64()]
	}

	return string(word)
}

// Process перебирает комбинации фиксированной длины maxLength в своём диапазоне.
func Process(hash, algorithm, alphabet string, maxLength, partNumber, partCount int) ([]string, uint64, error) {
	var found []string
	var checked uint64

	if maxLength <= 0 || len(alphabet) == 0 || partCount <= 0 {
		return found, checked, nil
	}

	for length := 1; length <= maxLength; length++ {
		N := bigPow(int64(len(alphabet)), length)

		bPartCount := big.NewInt(int64(partCount))
		bPartNumber := big.NewInt(int64(partNumber))

		start := new(big.Int).Mul(N, bPartNumber)
		start.Div(start, bPartCount)

		next := new(big.Int).Add(bPartNumber, big.NewInt(1))
		end := new(big.Int).Mul(N, next)
		end.Div(end, bPartCount)

		one := big.NewInt(1)
		for i := new(big.Int).Set(start); i.Cmp(end) < 0; i.Add(i, one) {
			word := indexToWordBig(i, alphabet, length)
			checked++
			if checkHash(word, hash, algorithm) {
				found = append(found, word)
			}
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
