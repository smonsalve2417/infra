package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

func GenerateRandomCode() (string, error) {
	const digits = "0123456789"
	code := make([]byte, 6)
	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		code[i] = digits[num.Int64()]
	}
	return string(code), nil
}
