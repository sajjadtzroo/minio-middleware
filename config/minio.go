package config

import (
	"github.com/gofiber/storage/minio"
	"os"
)

type MinIoConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
}

type MinIOClients struct {
	Storage *minio.Storage
}

func GetMinioCredentials() MinIoConfig {
	config := MinIoConfig{
		Endpoint:  os.Getenv("MINIO_ENDPOINT"),
		AccessKey: os.Getenv("MINIO_ACCESSKEY"),
		SecretKey: os.Getenv("MINIO_SECRETKEY"),
	}

	if len(config.Endpoint) == 0 {
		panic("MINIO_ENDPOINT is empty")
	}

	if len(config.AccessKey) == 0 {
		panic("MINIO_ACCESSKEY is empty")
	}

	if len(config.SecretKey) == 0 {
		panic("MINIO_SECRETKEY is empty")
	}

	return config
}

func GetMinIOClients(config MinIoConfig) MinIOClients {
	storage := minio.New(minio.Config{
		Secure:   true,
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
