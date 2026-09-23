package config

import (
	"os"
	"strconv"
)

func Env(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	return value
}

func EnvInt(key string, def int) int {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	number, err := strconv.Atoi(value)
	if err != nil {
		return def
	}

	return number
}
