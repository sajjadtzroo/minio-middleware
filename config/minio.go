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

	// Clean endpoint - remove protocols if present
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimSpace(endpoint)

	config := MinIoConfig{
		Endpoint: endpoint,
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
	} else {
		// Auto-detect SSL based on endpoint
		if strings.Contains(endpoint, "darkube.app") || strings.Contains(endpoint, ":443") {
			config.UseSSL = true
		} else if strings.Contains(endpoint, "localhost") || strings.Contains(endpoint, "127.0.0.1") {
			config.UseSSL = false
		} else {
			config.UseSSL = true // Default to true for production
		}
	}

	// Validation with better error messages
	if len(config.Endpoint) == 0 {
		log.Printf("❌ MINIO_ENDPOINT is empty")
		panic("MINIO_ENDPOINT environment variable is required")
	}

	if len(config.AccessKey) == 0 {
		log.Printf("❌ MINIO_ACCESSKEY or MINIO_ACCESS_KEY is empty")
		panic("MinIO access key is required (MINIO_ACCESSKEY or MINIO_ACCESS_KEY)")
	}

	if len(config.SecretKey) == 0 {
		log.Printf("❌ MINIO_SECRETKEY or MINIO_SECRET_KEY is empty")
		panic("MinIO secret key is required (MINIO_SECRETKEY or MINIO_SECRET_KEY)")
	}

	log.Printf("✅ MinIO Config: endpoint=%s, ssl=%v", config.Endpoint, config.UseSSL)

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

	// Test connection
	if storage.Conn() == nil {
		log.Printf("⚠️ Warning: MinIO connection could not be established")
	} else {
		log.Printf("✅ MinIO client created successfully")
	}

	return MinIOClients{
		Storage: storage,
	}
}