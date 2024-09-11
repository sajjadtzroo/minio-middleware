package config

import "os"

func GetJwtKey() string {
	key := os.Getenv("JWT_KEY")
	if len(key) == 0 {
		panic("JWT_KEY is empty")
	}

	return key
}
