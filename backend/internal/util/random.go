package util

import (
	"crypto/rand"
	"math/big"
)

func RandomString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			panic(err) // Should not happen
		}
		result[i] = chars[num.Int64()]
	}
	return string(result)
}
