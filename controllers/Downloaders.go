package controllers

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/pkg/instagram_api"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

func DownloadFile(ctx *fiber.Ctx) error {
	ctx.SetUserContext(context.Background())

	reqPath := strings.Split(ctx.Path(), "/")
	if len(reqPath) < 4 {
		if reqPath[2] == "favicon.ico" {
			return ctx.SendStatus(404)
		}

		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "Invalid path",
		})
	}

	bucket := reqPath[2]
	reqPath = slices.Delete(reqPath, 0, 3)

	path := strings.Join(reqPath, "/")

	minioClient := ctx.Locals("minio").(*config.MinIOClients)
	objectInfo := minioClient.Storage.Conn().ListObjects(ctx.UserContext(), bucket, minio.ListObjectsOptions{
		Prefix:    path,
		Recursive: true,
		UseV1:     true,
	})

	for info := range objectInfo {
		if info.Size > 0 {
			if strings.Split(info.Key, ".")[0] == path {
				object, err := minioClient.Storage.Conn().GetObject(ctx.UserContext(), bucket, info.Key, minio.GetObjectOptions{})
				if err != nil {
					return ctx.Status(500).JSON(models.GenericResponse{
						Result:  false,
						Message: err.Error(),
					})
				}

				data, _ := io.ReadAll(object)
				ctx.Set("Content-Type", http.DetectContentType(data))
				_ = object.Close()
				return ctx.Status(200).Send(data)
			}
		}
	}

	return ctx.Status(404).JSON(models.GenericResponse{
		Result:  false,
		Message: "File Not Found",
	})
}

type TelegramProfileResponse struct {
	Description  string      `json:"description"`
	Id           int64       `json:"id"`
	MemberCount  int         `json:"member_count"`
	ProfilePhoto string      `json:"profile_photo"`
	Result       bool        `json:"result"`
	Title        string      `json:"title"`
	Username     interface{} `json:"username"`
}

func DownloadProfile(ctx *fiber.Ctx) error {
	ctx.SetUserContext(context.Background())

	pk := ctx.Params("pk")
	media := ctx.Params("media")
	userName := ctx.Params("username")

	minioClient := ctx.Locals("minio").(*config.MinIOClients)
	var bucketName string
	if media == "telegram" {
		bucketName = "profile-telegram"
	} else {
		bucketName = "profile-instagram"
	}

	minioListCtx, cancelMinIOList := context.WithTimeout(ctx.UserContext(), 30*time.Second)
	defer cancelMinIOList()
	objectInfo := minioClient.Storage.Conn().ListObjects(minioListCtx, bucketName, minio.ListObjectsOptions{
		Prefix:    pk,
		Recursive: true,
		UseV1:     true,
	})

	for info := range objectInfo {
		if info.Size > 0 {
			if strings.Split(info.Key, ".")[0] == pk {
				minioGetCtx, cancelMinIOGet := context.WithTimeout(ctx.UserContext(), 30*time.Second)
				object, err := minioClient.Storage.Conn().GetObject(minioGetCtx, bucketName, info.Key, minio.GetObjectOptions{})
				if err != nil {
					cancelMinIOGet()
					return ctx.Status(500).JSON(models.GenericResponse{
						Result:  false,
						Message: err.Error(),
					})
				}

				data, err := io.ReadAll(object)
				if err != nil {
					cancelMinIOGet()
					return ctx.Status(500).JSON(models.GenericResponse{
						Result:  false,
						Message: err.Error(),
					})
				}
				ctx.Set("Content-Type", http.DetectContentType(data))
				_ = object.Close()
				cancelMinIOGet()
				return ctx.Status(200).Send(data)
			}
		}
	}

	if media == "telegram" {
		var reqUrl string
		if strings.HasPrefix(userName, "@") {
			reqUrl = "https://tgobserver.darkube.app/getChannelInfo?channel=" + userName
		} else {
			reqUrl = "https://tgobserver.darkube.app/getChannelInfo?channel_link=" + userName
		}

		telegramReqCtx, cancelTelegramCtx := context.WithTimeout(ctx.UserContext(), 60*time.Second)
		defer cancelTelegramCtx()
		req, err := http.NewRequestWithContext(telegramReqCtx, "GET", reqUrl, nil)
		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		httpClient := http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 60 * time.Second,
		}
		res, err := httpClient.Do(req)
		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: string(body),
			})
		}

		var telegramProfile TelegramProfileResponse
		err = json.Unmarshal(body, &telegramProfile)
		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		photoReqCtx, cancelPhotoReq := context.WithTimeout(ctx.UserContext(), 60*time.Second)
		defer cancelPhotoReq()
		photoReq, err := http.NewRequestWithContext(photoReqCtx, "GET", "https://tgobserver.darkube.app"+telegramProfile.ProfilePhoto, nil)
		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		httpClientPhotoReq := http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
		photoRes, err := httpClientPhotoReq.Do(photoReq)
		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		if photoRes.StatusCode != 200 {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: "Failed to get profile photo from tgObserver",
			})
		}

		responseFileBody, err := io.ReadAll(photoRes.Body)
		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		mimeType := http.DetectContentType(responseFileBody)
		minioPutObjectCtx, cancelMinioPutObject := context.WithTimeout(ctx.UserContext(), 60*time.Second)
		defer cancelMinioPutObject()
		file := bytes.NewReader(responseFileBody)
		_, err = minioClient.Storage.Conn().PutObject(
			minioPutObjectCtx,
			bucketName,
			pk+".jpg",
			file,
			file.Size(),
			minio.PutObjectOptions{},
		)

		if err != nil {
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: err.Error(),
			})
		}

		ctx.Set("Content-Type", mimeType)
		return ctx.Status(200).Send(responseFileBody)
	}

	instaApi := ctx.Locals("INSTAGRAM_API").(*instagram_api.InstagramApi)
	profilePicUrl, err := instaApi.GetProfile(userName)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	snitchConfig := ctx.Locals("SNITCH_CONFIG").(*config.SnitchConfiguration)
	instagramReqCtx, cancelInstagramReq := context.WithTimeout(ctx.UserContext(), 60*time.Second)
	defer cancelInstagramReq()

	log.Printf("Sntich Proxy Url: %v", snitchConfig.Url)

	var requestURL = ""
	if len(snitchConfig.Url) == 0 {
		requestURL = profilePicUrl
	} else {
		requestURL = snitchConfig.Url
	}

	if requestURL == "" {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Empty request URL: both profile picture URL and snitch URL are empty",
		})
	}

	req, err := http.NewRequestWithContext(instagramReqCtx, "GET", requestURL, nil)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	log.Printf("Image-URL: %v", profilePicUrl)

	if requestURL != profilePicUrl {
		log.Printf("Use-Snitcher")
		req.Header.Set("X-Proxy-To", profilePicUrl)
	}

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	picRes, err := httpClient.Do(req)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(picRes.Body)
	bodyRaw, _ := io.ReadAll(picRes.Body)

	if picRes.StatusCode != 200 {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: string(bodyRaw),
		})
	}

	mimeType := http.DetectContentType(bodyRaw)
	putObjectCtx, cancelPutObject := context.WithTimeout(ctx.UserContext(), 60*time.Second)
	defer cancelPutObject()
	file := bytes.NewReader(bodyRaw)
	_, err = minioClient.Storage.Conn().PutObject(
		putObjectCtx,
		bucketName,
		pk+".jpg",
		file,
		file.Size(),
		minio.PutObjectOptions{},
	)

	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	ctx.Set("Content-Type", mimeType)
	return ctx.Status(200).Send(bodyRaw)

}

func ZipMultipleFiles(ctx *fiber.Ctx) error {
	contentType := ctx.Get("Content-Type")
	if contentType != "text/plain" {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: fmt.Sprintf("Content-Type %s not supported", contentType),
		})
	}

	bodyBase64 := ctx.Body()
	bodyRaw, err := base64.StdEncoding.DecodeString(string(bodyBase64))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	var requestData [][]string
	if err = json.Unmarshal(bodyRaw, &requestData); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	// Validate input data early - حداقل 2 المان باید باشه (botName, fileId)
	// المان سوم (username) اختیاری هست
	for _, data := range requestData {
		if len(data) < 2 {
			return ctx.Status(fiber.StatusBadRequest).JSON(models.GenericResponse{
				Result:  false,
				Message: "data format error: each item needs at least [botName, fileId]",
			})
		}
	}

	archiveName := fmt.Sprintf("%x.zip", sha256.Sum256(bodyBase64))

	// Set response headers immediately for streaming
	ctx.Set("Content-Type", "application/zip")
	ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", archiveName))
	ctx.Set("Transfer-Encoding", "chunked")

	// Create a pipe for streaming the zip directly to the response
	pipeReader, pipeWriter := io.Pipe()
	zipWriter := zip.NewWriter(pipeWriter)

	// Channel for coordinating file download and zip writing
	type fileResult struct {
		fileID      string
		fileName    string // username برای نام‌گذاری فایل
		fileData    []byte
		contentType string
		extension   string
		err         error
	}

	fileResultChan := make(chan fileResult, len(requestData))
	var downloadWg sync.WaitGroup

	// Start concurrent downloads
	for _, data := range requestData {
		botName := data[0]
		fileID := data[1]

		// اگه username داده شده از اون استفاده کن، وگرنه از fileID
		fileName := fileID
		if len(data) >= 3 && data[2] != "" {
			fileName = data[2]
		}

		botAPIs := selectBotAPI(ctx, strings.ToLower(botName))

		downloadWg.Add(1)
		go func(fileID, fileName, botName string) {
			defer downloadWg.Done()

			// Download file with racing
			filePath, selectedBotAPI, err := raceGetFile(botAPIs, fileID)
			if err != nil {
				fileResultChan <- fileResult{fileID: fileID, fileName: fileName, err: err}
				return
			}

			fileData, resContentType, err := raceDownloadFile(botAPIs, selectedBotAPI.Explode(filePath.(string)))
			if err != nil {
				fileResultChan <- fileResult{fileID: fileID, fileName: fileName, err: err}
				return
			}

			// Determine file extension
			mimeType := http.DetectContentType(fileData)
			if strings.Contains(mimeType, "text/plain") {
				mimeType = resContentType
			}

			fileExtension := strings.Split(mimeType, "/")[1]

			fileResultChan <- fileResult{
				fileID:      fileID,
				fileName:    fileName,
				fileData:    fileData,
				contentType: mimeType,
				extension:   fileExtension,
				err:         nil,
			}
		}(fileID, fileName, botName)
	}

	// Goroutine to close the channel when all downloads are done
	go func() {
		downloadWg.Wait()
		close(fileResultChan)
	}()

	// Goroutine to write files to zip as they become available
	go func() {
		defer func() {
			_ = zipWriter.Close()
			_ = pipeWriter.Close()
		}()

		filesProcessed := 0
		for result := range fileResultChan {
			if result.err != nil {
				log.Printf("Error downloading file %s: %v", result.fileID, result.err)
				// Write error to pipe to terminate the stream
				_ = pipeWriter.CloseWithError(result.err)
				return
			}

			// Create zip entry and write file data - استفاده از fileName به جای fileID
			zipFileWriter, err := zipWriter.Create(result.fileName + "." + result.extension)
			if err != nil {
				log.Printf("Error creating zip entry for %s: %v", result.fileName, err)
				_ = pipeWriter.CloseWithError(err)
				return
			}

			if _, err := zipFileWriter.Write(result.fileData); err != nil {
				log.Printf("Error writing file data for %s: %v", result.fileName, err)
				_ = pipeWriter.CloseWithError(err)
				return
			}

			filesProcessed++
			log.Printf("Added file %s (ID: %s) to zip (%d/%d)", result.fileName, result.fileID, filesProcessed, len(requestData))
		}

		log.Printf("Successfully processed all %d files for zip", filesProcessed)
	}()

	// Stream the zip data directly to the client
	ctx.Context().SetBodyStream(pipeReader, -1)
	return nil
}

// ZipMultipleFilesOptimized is a high-performance version with additional optimizations
func ZipMultipleFilesOptimized(ctx *fiber.Ctx) error {
	contentType := ctx.Get("Content-Type")
	if contentType != "text/plain" {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: fmt.Sprintf("Content-Type %s not supported", contentType),
		})
	}

	bodyBase64 := ctx.Body()
	bodyRaw, err := base64.StdEncoding.DecodeString(string(bodyBase64))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	var requestData [][]string
	if err = json.Unmarshal(bodyRaw, &requestData); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	// Validate input data early - حداقل 2 المان باید باشه (botName, fileId)
	// المان سوم (username) اختیاری هست
	for _, data := range requestData {
		if len(data) < 2 {
			return ctx.Status(fiber.StatusBadRequest).JSON(models.GenericResponse{
				Result:  false,
				Message: "data format error: each item needs at least [botName, fileId]",
			})
		}
	}

	// Limit concurrent downloads to prevent resource exhaustion
	const maxConcurrentDownloads = 10
	semaphore := make(chan struct{}, maxConcurrentDownloads)

	archiveName := fmt.Sprintf("%x.zip", sha256.Sum256(bodyBase64))

	// Set response headers immediately for streaming
	ctx.Set("Content-Type", "application/zip")
	ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", archiveName))
	ctx.Set("Transfer-Encoding", "chunked")
	ctx.Set("Cache-Control", "no-cache")

	// Create a buffered pipe for better performance
	pipeReader, pipeWriter := io.Pipe()

	// Use buffer for zip writer to improve I/O performance
	bufferedWriter := &bytes.Buffer{}
	zipWriter := zip.NewWriter(io.MultiWriter(pipeWriter, bufferedWriter))

	// Channel for coordinating file download and zip writing
	type fileResult struct {
		fileID      string
		fileName    string // username برای نام‌گذاری فایل
		fileData    []byte
		contentType string
		extension   string
		size        int64
		err         error
	}

	fileResultChan := make(chan fileResult, len(requestData))
	var downloadWg sync.WaitGroup

	// Track download progress
	downloadProgress := &sync.Map{}
	totalFiles := len(requestData)

	log.Printf("Starting ZIP creation for %d files", totalFiles)

	// Start concurrent downloads with rate limiting
	for i, data := range requestData {
		botName := data[0]
		fileID := data[1]

		// اگه username داده شده از اون استفاده کن، وگرنه از fileID
		fileName := fileID
		if len(data) >= 3 && data[2] != "" {
			fileName = data[2]
		}

		downloadWg.Add(1)
		go func(fileID, fileName, botName string, index int) {
			defer downloadWg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			downloadProgress.Store(fileID, "downloading")
			log.Printf("Starting download %d/%d: %s (name: %s)", index+1, totalFiles, fileID, fileName)

			botAPIs := selectBotAPI(ctx, strings.ToLower(botName))
			if len(botAPIs) == 0 {
				fileResultChan <- fileResult{fileID: fileID, fileName: fileName, err: fmt.Errorf("no bot APIs available for %s", botName)}
				return
			}

			// Download with timeout and retry logic
			var fileData []byte
			var resContentType string
			var downloadErr error

			// Try up to 2 times
			for attempt := 1; attempt <= 2; attempt++ {
				// Get file path with racing
				filePath, selectedBotAPI, err := raceGetFile(botAPIs, fileID)
				if err != nil {
					downloadErr = err
					if attempt == 2 {
						break
					}
					log.Printf("Attempt %d failed for %s, retrying: %v", attempt, fileID, err)
					time.Sleep(time.Millisecond * 100)
					continue
				}

				// Download file data with racing
				fileData, resContentType, err = raceDownloadFile(botAPIs, selectedBotAPI.Explode(filePath.(string)))
				if err != nil {
					downloadErr = err
					if attempt == 2 {
						break
					}
					log.Printf("Download attempt %d failed for %s, retrying: %v", attempt, fileID, err)
					time.Sleep(time.Millisecond * 100)
					continue
				}

				// Success
				downloadErr = nil
				break
			}

			if downloadErr != nil {
				downloadProgress.Store(fileID, "failed")
				fileResultChan <- fileResult{fileID: fileID, fileName: fileName, err: downloadErr}
				return
			}

			// Determine file extension
			mimeType := http.DetectContentType(fileData)
			if strings.Contains(mimeType, "text/plain") && resContentType != "" {
				mimeType = resContentType
			}

			fileExtension := "bin" // default
			if parts := strings.Split(mimeType, "/"); len(parts) == 2 {
				fileExtension = parts[1]
			}

			downloadProgress.Store(fileID, "completed")
			log.Printf("Download completed %d/%d: %s as %s (%d bytes)", index+1, totalFiles, fileID, fileName, len(fileData))

			fileResultChan <- fileResult{
				fileID:      fileID,
				fileName:    fileName,
				fileData:    fileData,
				contentType: mimeType,
				extension:   fileExtension,
				size:        int64(len(fileData)),
				err:         nil,
			}
		}(fileID, fileName, botName, i)
	}

	// Goroutine to close the channel when all downloads are done
	go func() {
		downloadWg.Wait()
		close(fileResultChan)
	}()

	// Goroutine to write files to zip as they become available
	zipWriteComplete := make(chan error, 1)
	go func() {
		defer func() {
			if err := zipWriter.Close(); err != nil {
				log.Printf("Error closing zip writer: %v", err)
			}
			if err := pipeWriter.Close(); err != nil {
				log.Printf("Error closing pipe writer: %v", err)
			}
		}()

		filesProcessed := 0
		totalSize := int64(0)

		for result := range fileResultChan {
			if result.err != nil {
				log.Printf("Error downloading file %s: %v", result.fileID, result.err)
				zipWriteComplete <- result.err
				return
			}

			// Create zip entry with optimized compression - استفاده از fileName
			zipFileWriter, err := zipWriter.CreateHeader(&zip.FileHeader{
				Name:   result.fileName + "." + result.extension,
				Method: zip.Deflate, // Use compression
			})
			if err != nil {
				log.Printf("Error creating zip entry for %s: %v", result.fileName, err)
				zipWriteComplete <- err
				return
			}

			// Write file data in chunks for better memory usage
			chunkSize := 32 * 1024 // 32KB chunks
			reader := bytes.NewReader(result.fileData)
			written, err := io.CopyBuffer(zipFileWriter, reader, make([]byte, chunkSize))
			if err != nil {
				log.Printf("Error writing file data for %s: %v", result.fileName, err)
				zipWriteComplete <- err
				return
			}

			filesProcessed++
			totalSize += result.size
			log.Printf("Added file %s (ID: %s) to zip (%d/%d) - %d bytes, total: %d bytes",
				result.fileName, result.fileID, filesProcessed, totalFiles, written, totalSize)
		}

		log.Printf("Successfully processed all %d files for zip, total size: %d bytes", filesProcessed, totalSize)
		zipWriteComplete <- nil
	}()

	// Monitor the zip writing process
	go func() {
		if err := <-zipWriteComplete; err != nil {
			_ = pipeWriter.CloseWithError(err)
		}
	}()

	// Stream the zip data directly to the client
	ctx.Context().SetBodyStream(pipeReader, -1)
	return nil
}

// ZipPerformanceStats tracks performance metrics for ZIP operations
type ZipPerformanceStats struct {
	StartTime       time.Time `json:"start_time"`
	TotalFiles      int       `json:"total_files"`
	FilesProcessed  int       `json:"files_processed"`
	TotalSize       int64     `json:"total_size"`
	AverageFileSize int64     `json:"average_file_size"`
	Duration        string    `json:"duration"`
	ThroughputMBps  float64   `json:"throughput_mbps"`
	SuccessRate     float64   `json:"success_rate"`
	ConcurrentLimit int       `json:"concurrent_limit"`
}

// GetZipPerformanceInfo returns performance information for ZIP operations
func GetZipPerformanceInfo(ctx *fiber.Ctx) error {
	// This would be expanded to track actual performance metrics
	// For now, it returns configuration information
	stats := ZipPerformanceStats{
		StartTime:       time.Now(),
		ConcurrentLimit: 10, // Current limit from optimized version
	}

	return ctx.JSON(fiber.Map{
		"result": true,
		"stats":  stats,
		"recommendations": []string{
			"Use /zip/multi/optimized for better performance",
			"Limit concurrent requests to prevent resource exhaustion",
			"Consider implementing caching for frequently requested files",
			"Monitor memory usage with large file sets",
		},
		"data_format": []string{
			"[botName, fileId] - اسم فایل با fileId ذخیره میشه",
			"[botName, fileId, username] - اسم فایل با username ذخیره میشه",
		},
	})
}