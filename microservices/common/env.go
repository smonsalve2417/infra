package common

import "syscall"

func GetEnv(key string, fallback string) string {
	if value, ok := syscall.Getenv(key); ok {
		return value
	}

	return fallback
}
