package controllers

import (
	"go-uploader/pkg/telegram_api"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// raceGetFileResult holds the result of a bot API GetFile operation
type raceGetFileResult struct {
	filePath interface{}
	err      error
	botAPI   *telegram_api.TelegramAPI
}

// raceUploadResult holds the result of a bot API UploadFile operation
type raceUploadResult struct {
	fileId string
	err    error
	botAPI *telegram_api.TelegramAPI
}

// raceDownloadResult holds the result of a bot API DownloadFile operation
type raceDownloadResult struct {
	fileData    []byte
	contentType string
	err         error
	botAPI      *telegram_api.TelegramAPI
}

// raceGetFile attempts to get file info from multiple bot APIs concurrently
func raceGetFile(botAPIs []*telegram_api.TelegramAPI, fileId string) (interface{}, *telegram_api.TelegramAPI, error) {
	if len(botAPIs) == 0 {
		return nil, nil, fiber.NewError(500, "No bot APIs available")
	}

	resultChan := make(chan raceGetFileResult, len(botAPIs))
	var wg sync.WaitGroup

	for _, botAPI := range botAPIs {
		wg.Add(1)
		go func(api *telegram_api.TelegramAPI) {
			defer wg.Done()
			filePath, err := api.GetFile(fileId)
			resultChan <- raceGetFileResult{
				filePath: filePath,
				err:      err,
				botAPI:   api,
			}
		}(botAPI)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first successful result
	for result := range resultChan {
		if result.err == nil {
			return result.filePath, result.botAPI, nil
		}
	}

	return nil, nil, fiber.NewError(500, "All bot APIs failed to get file")
}

// raceDownloadFile attempts to download file from multiple bot APIs concurrently
func raceDownloadFile(botAPIs []*telegram_api.TelegramAPI, filePathString string) ([]byte, string, error) {
	if len(botAPIs) == 0 {
		return nil, "", fiber.NewError(500, "No bot APIs available")
	}

	resultChan := make(chan raceDownloadResult, len(botAPIs))
	var wg sync.WaitGroup

	for _, botAPI := range botAPIs {
		wg.Add(1)
		go func(api *telegram_api.TelegramAPI) {
			defer wg.Done()
			fileData, contentType, err := api.DownloadFile(filePathString)
			resultChan <- raceDownloadResult{
				fileData:    fileData,
				contentType: contentType,
				err:         err,
				botAPI:      api,
			}
		}(botAPI)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first successful result
	for result := range resultChan {
		if result.err == nil {
			return result.fileData, result.contentType, nil
		}
	}

	return nil, "", fiber.NewError(500, "All bot APIs failed to download file")
}

// raceUploadFile attempts to upload file to multiple bot APIs concurrently
func raceUploadFile(botAPIs []*telegram_api.TelegramAPI, contentType, filename string, data []byte, destChatId string) (string, error) {
	if len(botAPIs) == 0 {
		return "", fiber.NewError(500, "No bot APIs available")
	}

	resultChan := make(chan raceUploadResult, len(botAPIs))
	var wg sync.WaitGroup

	for _, botAPI := range botAPIs {
		wg.Add(1)
		go func(api *telegram_api.TelegramAPI) {
			defer wg.Done()
			fileId, err := api.UploadFile(contentType, filename, data, destChatId)
			resultChan <- raceUploadResult{
				fileId: fileId,
				err:    err,
				botAPI: api,
			}
		}(botAPI)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first successful result
	for result := range resultChan {
		if result.err == nil {
			return result.fileId, nil
		}
	}

	return "", fiber.NewError(500, "All bot APIs failed to upload file")
}
