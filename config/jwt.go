package config

import (
	"log"
	"os"
)

func GetJwtKey() string {
	key := os.Getenv("JWT_KEY")

	if len(key) == 0 {
		key = os.Getenv("JWT_SECRET")
	}

	if len(key) == 0 {
		log.Fatal("JWT_KEY or JWT_SECRET environment variable must be set")
	}

	return key
}
