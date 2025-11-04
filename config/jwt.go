package config

import (
	"log"
	"os"
)

func GetJwtKey() string {
	// Try JWT_KEY first (old format)
	key := os.Getenv("JWT_KEY")

	// If not found, try JWT_SECRET (new format)
	if len(key) == 0 {
		key = os.Getenv("JWT_SECRET")
	}

	// If still not found, use a default for development
	if len(key) == 0 {
		log.Printf("⚠️ WARNING: JWT_KEY not set, using default (NOT SAFE FOR PRODUCTION)")
		key = "default-jwt-key-not-for-production"
	}

	return key
}