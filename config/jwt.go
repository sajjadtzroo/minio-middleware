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

	if len(key) < 16 {
		log.Printf("⚠️ WARNING: JWT_KEY is shorter than 16 characters, this is insecure")
	}

	return key
}
