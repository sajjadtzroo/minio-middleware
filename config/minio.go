package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/gofiber/storage/minio"
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
	endpoint := os.Getenv("MINIO_ENDPOINT")

	// Clean endpoint - remove http:// or https:// if present
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimSpace(endpoint)

	config := MinIoConfig{
		Endpoint: endpoint,
		UseSSL:   false, // default to false
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

	// Parse UseSSL from environment
	if useSSL := os.Getenv("MINIO_USE_SSL"); useSSL != "" {
		config.UseSSL, _ = strconv.ParseBool(useSSL)
	}

	// If endpoint contains port 443, assume SSL
	if strings.Contains(endpoint, ":443") {
		config.UseSSL = true
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

	log.Printf("MinIO Configuration:")
	log.Printf("  Endpoint: %s", config.Endpoint)
	log.Printf("  UseSSL: %v", config.UseSSL)
	log.Printf("  AccessKey: %s***", config.AccessKey[:3])

	return config
}

func GetMinIOClients(config MinIoConfig) MinIOClients {
	// Try to create the storage client
	storage := minio.New(minio.Config{
		Bucket:   "", // Leave empty, we specify bucket per operation
		Endpoint: config.Endpoint,
		Secure:   config.UseSSL,
		Credentials: minio.Credentials{
			AccessKeyID:     config.AccessKey,
			SecretAccessKey: config.SecretKey,
		},
		Reset:           false, // Important: don't reset on each request
		CacheTTL:        3600,  // Cache for 1 hour
		RequestTimeout:  30,    // 30 seconds timeout
	})

	// Test connection
	conn := storage.Conn()
	if conn == nil {
		panic("Failed to create MinIO connection")
	}

	log.Printf("✅ MinIO client created successfully")

	return MinIOClients{
		Storage: storage,
	}
}