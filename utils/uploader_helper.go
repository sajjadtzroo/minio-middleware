package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"slices"
	"time"
)

var ValidBuckets = []string{"instagram", "telegram", "influencer", "tracker"}

var (
	ImageFileTypes = map[string]string{
		"image/jpeg": "jpeg",
		"image/jpg":  "jpg",
		"image/png":  "png",
	}
)

func CreateFileID(fileBuffer []byte) ([]byte, error) {
	h := sha256.New()

	// Use nanosecond timestamp for better uniqueness
	if _, err := h.Write([]byte(time.Now().Format(time.RFC3339Nano))); err != nil {
		return nil, err
	}

	// Add random bytes to prevent collisions
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, err
	}
	if _, err := h.Write(randomBytes); err != nil {
		return nil, err
	}

	if _, err := h.Write(fileBuffer); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func IsValidBucket(name string) bool {
	return slices.Contains(ValidBuckets, name)
}

func CreateFilePath(fileName string, ext string) string {
	if len(ext) != 0 {
		fileName += "." + ext
	}

	return fileName
}
