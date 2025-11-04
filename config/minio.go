package config

import (
	"github.com/gofiber/storage/minio"
	"os"
	"strconv"
)

type MinIoConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

type MinIOClients struct {
	Storage *minio.Storage
}

func GetMinioCredentials() MinIoConfig {
	config := MinIoConfig{
		Endpoint: os.Getenv("MINIO_ENDPOINT"),
		UseSSL:   false, // default
	}

	// Try both formats for AccessKey
	config.AccessKey = os.Getenv("MINIO_ACCESSKEY")
	if len(config.AccessKey) == 0 {
		config.AccessKey = os.Getenv("MINIO_ACCESS_KEY")
	}

	// Try both formats for SecretKey
	config.SecretKey = os.Getenv("MINIO_SECRETKEY")
	if len(config.SecretKey) == 0 {
		config.SecretKey = os.Getenv("MINIO_SECRET_KEY")
	}

	// Parse UseSSL from environment (optional)
	if useSSL := os.Getenv("MINIO_USE_SSL"); useSSL != "" {
		config.UseSSL, _ = strconv.ParseBool(useSSL)
	}

	if len(config.Endpoint) == 0 {
		panic("MINIO_ENDPOINT is empty")
	}

	if len(config.AccessKey) == 0 {
		panic("MINIO_ACCESSKEY or MINIO_ACCESS_KEY is empty")
	}

	if len(config.SecretKey) == 0 {
		panic("MINIO_SECRETKEY or MINIO_SECRET_KEY is empty")
	}

	return config
}

func GetMinIOClients(config MinIoConfig) MinIOClients {
	storage := minio.New(minio.Config{
		Secure:   config.UseSSL,
		Endpoint: config.Endpoint,
		Credentials: minio.Credentials{
			AccessKeyID:     config.AccessKey,
			SecretAccessKey: config.SecretKey,
		},
	})

	return MinIOClients{
		Storage: storage,
	}
}