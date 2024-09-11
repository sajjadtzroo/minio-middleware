package utils

import (
	"mime"
	"net/http"
)

func GetMimeType(fileData []byte) string {
	return http.DetectContentType(fileData)
}

func GetExtensionFromMimeType(mimeType string) ([]string, error) {
	return mime.ExtensionsByType(mimeType)
}
