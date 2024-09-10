package utils

import (
	"crypto/sha256"
	"time"
)

type FileDetails struct {
	Format string
	Id     string
}

const (
	MaxImageSize = 11 * 1024 * 1024 // 11MB
	MinImageSize = 10 * 1000        // 10 KB
)

var (
	ImageAllowedFormats = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}

	ImageFileTypes = map[string]string{
		"image/jpeg": "jpeg",
		"image/jpg":  "jpg",
		"image/png":  "png",
	}
)

func CreateFileID(fileBuffer []byte) ([]byte, error) {
	h := sha256.New()
	var err error

	_, err = h.Write([]byte(time.Now().Format(time.RFC3339)))
	if err != nil {
		return nil, err
	}

	h.Write(fileBuffer)
	return h.Sum(nil), nil
}

func CreateFilePath(fileName string, ext string) string {
	if len(ext) != 0 {
		fileName += "." + ext
	}

	return fileName
}
