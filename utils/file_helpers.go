package utils

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
)

func GetMimeType(fileData []byte) string {
	return http.DetectContentType(fileData)
}

func GetExtensionFromMimeType(mimeType string) ([]string, error) {
	return mime.ExtensionsByType(mimeType)
}

func OpenFile(file *multipart.FileHeader) (*bytes.Buffer, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func(src multipart.File) {
		_ = src.Close()
	}(src)

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, src); err != nil {
		return nil, err
	}

	return buf, nil
}
