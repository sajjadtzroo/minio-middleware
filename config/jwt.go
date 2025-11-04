package config

import "os"

func GetJwtKey() string {
	// First try JWT_KEY (old format), then JWT_SECRET (new format)
	key := os.Getenv("JWT_KEY")
	if len(key) == 0 {
		key = os.Getenv("JWT_SECRET")
	}

	if len(key) == 0 {
		panic("JWT_KEY or JWT_SECRET is empty")
	}

	return key
}