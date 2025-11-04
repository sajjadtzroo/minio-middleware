package controllers

import (
	"context"
	"go-uploader/config"
	"go-uploader/pkg/telegram_api"
	"log"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// raceGetFileResult holds the result of a bot API GetFile operation
type raceGetFileResult struct {
	filePath interface{}
	err      error
	botAPI   *telegram_api.TelegramAPI
	botName  string
}

// raceUploadResult holds the result of a bot API UploadFile operation
type raceUploadResult struct {
	fileId  string
	err     error
	botAPI  *telegram_api.TelegramAPI
	botName string
}

// raceDownloadResult holds the result of a bot API DownloadFile operation
type raceDownloadResult struct {
	fileData    []byte
	contentType string
	err         error
	botAPI      *telegram_api.TelegramAPI
	botName     string
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
				botName:  "unknown",
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
			log.Printf("✅ GetFile successful using bot: %s (FileID: %s)", result.botAPI.String(), fileId)
			return result.filePath, result.botAPI, nil
		}
	}

	log.Printf("❌ All bots failed to GetFile for FileID: %s", fileId)
	return nil, nil, fiber.NewError(500, "All bot APIs failed to get file")
}

// raceGetFileWithNames attempts to get file info from multiple named bots concurrently
func raceGetFileWithNames(namedBots []config.NamedBot, fileId string) (interface{}, *telegram_api.TelegramAPI, string, error) {
	if len(namedBots) == 0 {
		return nil, nil, "", fiber.NewError(500, "No named bots available")
	}

	log.Printf("🏁 Starting GetFile race with %d bots for FileID: %s", len(namedBots), fileId)

	resultChan := make(chan raceGetFileResult, len(namedBots))
	var wg sync.WaitGroup

	for _, namedBot := range namedBots {
		wg.Add(1)
		go func(bot config.NamedBot) {
			defer wg.Done()
			log.Printf("🚀 Bot '%s' attempting GetFile for FileID: %s", bot.Name, fileId)
			filePath, err := bot.API.GetFile(fileId)
			resultChan <- raceGetFileResult{
				filePath: filePath,
				err:      err,
				botAPI:   bot.API,
				botName:  bot.Name,
			}
		}(namedBot)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first successful result
	for result := range resultChan {
		if result.err == nil {
			log.Printf("🏆 GetFile WON by bot: '%s' (%s) for FileID: %s", result.botName, result.botAPI.String(), fileId)
			return result.filePath, result.botAPI, result.botName, nil
		} else {
			log.Printf("❌ Bot '%s' failed GetFile: %v", result.botName, result.err)
		}
	}

	log.Printf("💥 All %d bots failed GetFile for FileID: %s", len(namedBots), fileId)
	return nil, nil, "", fiber.NewError(500, "All named bots failed to get file")
}

// raceGetFileWithNamesOptimized - Optimized version with timeouts and limited bot count
func raceGetFileWithNamesOptimized(namedBots []config.NamedBot, fileId string) (interface{}, *telegram_api.TelegramAPI, string, error) {
	if len(namedBots) == 0 {
		return nil, nil, "", fiber.NewError(500, "No named bots available")
	}

	// Use only first 3 bots for speed
	maxBots := 3
	if len(namedBots) < maxBots {
		maxBots = len(namedBots)
	}
	activeBots := namedBots[:maxBots]

	log.Printf("🏁 Optimized GetFile with %d bots for FileID: %s", len(activeBots), fileId)

	resultChan := make(chan raceGetFileResult, len(activeBots))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, namedBot := range activeBots {
		go func(bot config.NamedBot) {
			// Each bot gets its own timeout
			botCtx, botCancel := context.WithTimeout(ctx, 5*time.Second)
			defer botCancel()

			log.Printf("🚀 Bot '%s' attempting GetFile for FileID: %s", bot.Name, fileId)

			// Use GetFileWithContext if available
			filePath, err := bot.API.GetFileWithContext(botCtx, fileId)

			select {
			case resultChan <- raceGetFileResult{
				filePath: filePath,
				err:      err,
				botAPI:   bot.API,
				botName:  bot.Name,
			}:
			case <-ctx.Done():
				log.Printf("⏱️ Bot '%s' GetFile cancelled (timeout)", bot.Name)
			}
		}(namedBot)
	}

	// Wait for first successful result
	for i := 0; i < len(activeBots); i++ {
		select {
		case result := <-resultChan:
			if result.err == nil {
				cancel() // Cancel other operations
				log.Printf("🏆 GetFile won by: '%s' (attempt %d/%d)", result.botName, i+1, len(activeBots))
				return result.filePath, result.botAPI, result.botName, nil
			}
			log.Printf("❌ Bot '%s' failed: %v", result.botName, result.err)
		case <-ctx.Done():
			log.Printf("⏱️ GetFile timeout after %d attempts", i)
			return nil, nil, "", fiber.NewError(500, "GetFile timeout")
		}
	}

	return nil, nil, "", fiber.NewError(500, "All bots failed to get file")
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
				botName:     "unknown",
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
			log.Printf("✅ DownloadFile successful using bot: %s", result.botAPI.String())
			return result.fileData, result.contentType, nil
		}
	}

	log.Printf("❌ All bots failed to DownloadFile for path: %s", filePathString)
	return nil, "", fiber.NewError(500, "All bot APIs failed to download file")
}

// raceDownloadFileWithNames attempts to download file from multiple named bots concurrently
func raceDownloadFileWithNames(namedBots []config.NamedBot, filePathString string) ([]byte, string, string, error) {
	if len(namedBots) == 0 {
		return nil, "", "", fiber.NewError(500, "No named bots available")
	}

	log.Printf("🏁 Starting DownloadFile race with %d bots for path: %s", len(namedBots), filePathString)

	resultChan := make(chan raceDownloadResult, len(namedBots))
	var wg sync.WaitGroup

	for _, namedBot := range namedBots {
		wg.Add(1)
		go func(bot config.NamedBot) {
			defer wg.Done()
			log.Printf("🚀 Bot '%s' attempting DownloadFile for path: %s", bot.Name, filePathString)
			fileData, contentType, err := bot.API.DownloadFile(filePathString)
			resultChan <- raceDownloadResult{
				fileData:    fileData,
				contentType: contentType,
				err:         err,
				botAPI:      bot.API,
				botName:     bot.Name,
			}
		}(namedBot)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first successful result
	for result := range resultChan {
		if result.err == nil {
			log.Printf("🏆 DownloadFile WON by bot: '%s' (%s) - Downloaded %d bytes", result.botName, result.botAPI.String(), len(result.fileData))
			return result.fileData, result.contentType, result.botName, nil
		} else {
			log.Printf("❌ Bot '%s' failed DownloadFile: %v", result.botName, result.err)
		}
	}

	log.Printf("💥 All %d bots failed DownloadFile for path: %s", len(namedBots), filePathString)
	return nil, "", "", fiber.NewError(500, "All named bots failed to download file")
}

// raceDownloadFileWithNamesOptimized - Optimized version with timeouts
func raceDownloadFileWithNamesOptimized(namedBots []config.NamedBot, filePathString string) ([]byte, string, string, error) {
	if len(namedBots) == 0 {
		return nil, "", "", fiber.NewError(500, "No named bots available")
	}

	// Use only first 2 bots for download
	maxBots := 2
	if len(namedBots) < maxBots {
		maxBots = len(namedBots)
	}
	activeBots := namedBots[:maxBots]

	log.Printf("🏁 Optimized DownloadFile with %d bots", len(activeBots))

	resultChan := make(chan raceDownloadResult, len(activeBots))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for _, namedBot := range activeBots {
		go func(bot config.NamedBot) {
			botCtx, botCancel := context.WithTimeout(ctx, 15*time.Second)
			defer botCancel()

			log.Printf("🚀 Bot '%s' attempting DownloadFile", bot.Name)

			fileData, contentType, err := bot.API.DownloadFileWithContext(botCtx, filePathString)

			select {
			case resultChan <- raceDownloadResult{
				fileData:    fileData,
				contentType: contentType,
				err:         err,
				botAPI:      bot.API,
				botName:     bot.Name,
			}:
			case <-ctx.Done():
				log.Printf("⏱️ Bot '%s' DownloadFile cancelled (timeout)", bot.Name)
			}
		}(namedBot)
	}

	// Wait for first successful result
	for i := 0; i < len(activeBots); i++ {
		select {
		case result := <-resultChan:
			if result.err == nil {
				cancel()
				log.Printf("🏆 DownloadFile won by: '%s' (%d bytes)", result.botName, len(result.fileData))
				return result.fileData, result.contentType, result.botName, nil
			}
			log.Printf("❌ Bot '%s' failed: %v", result.botName, result.err)
		case <-ctx.Done():
			log.Printf("⏱️ DownloadFile timeout after %d attempts", i)
			return nil, "", "", fiber.NewError(500, "DownloadFile timeout")
		}
	}

	return nil, "", "", fiber.NewError(500, "All bots failed to download")
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
				fileId:  fileId,
				err:     err,
				botAPI:  api,
				botName: "unknown",
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
			log.Printf("✅ UploadFile successful using bot: %s (FileID: %s)", result.botAPI.String(), result.fileId)
			return result.fileId, nil
		}
	}

	log.Printf("❌ All bots failed to UploadFile: %s", filename)
	return "", fiber.NewError(500, "All bot APIs failed to upload file")
}

// raceUploadFileWithNames attempts to upload file to multiple named bots concurrently
func raceUploadFileWithNames(namedBots []config.NamedBot, contentType, filename string, data []byte, destChatId string) (string, string, error) {
	if len(namedBots) == 0 {
		return "", "", fiber.NewError(500, "No named bots available")
	}

	log.Printf("🏁 Starting UploadFile race with %d bots for file: %s (%d bytes)", len(namedBots), filename, len(data))

	resultChan := make(chan raceUploadResult, len(namedBots))
	var wg sync.WaitGroup

	for _, namedBot := range namedBots {
		wg.Add(1)
		go func(bot config.NamedBot) {
			defer wg.Done()
			log.Printf("🚀 Bot '%s' attempting UploadFile: %s", bot.Name, filename)
			fileId, err := bot.API.UploadFile(contentType, filename, data, destChatId)
			resultChan <- raceUploadResult{
				fileId:  fileId,
				err:     err,
				botAPI:  bot.API,
				botName: bot.Name,
			}
		}(namedBot)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Return the first successful result
	for result := range resultChan {
		if result.err == nil {
			log.Printf("🏆 UploadFile WON by bot: '%s' (%s) - FileID: %s", result.botName, result.botAPI.String(), result.fileId)
			return result.fileId, result.botName, nil
		} else {
			log.Printf("❌ Bot '%s' failed UploadFile: %v", result.botName, result.err)
		}
	}

	log.Printf("💥 All %d bots failed UploadFile for: %s", len(namedBots), filename)
	return "", "", fiber.NewError(500, "All named bots failed to upload file")
}

// getSpecificNamedBot selects a specific named bot by name, defaults to "relic" or first available
func getSpecificNamedBot(namedBots []config.NamedBot, preferredBotName string) (config.NamedBot, error) {
	if len(namedBots) == 0 {
		return config.NamedBot{}, fiber.NewError(500, "No named bots available")
	}

	// If specific bot name is provided, try to find it
	if preferredBotName != "" {
		for _, namedBot := range namedBots {
			if namedBot.Name == preferredBotName {
				log.Printf("🎯 Selected specific bot: '%s' (%s)", namedBot.Name, namedBot.API.String())
				return namedBot, nil
			}
		}
		log.Printf("⚠️ Requested bot '%s' not found, falling back to default", preferredBotName)
	}

	// Default to "relic" if available
	for _, namedBot := range namedBots {
		if namedBot.Name == "relic" {
			log.Printf("🔰 Using default bot: 'relic' (%s)", namedBot.API.String())
			return namedBot, nil
		}
	}

	// Fall back to first available bot
	firstBot := namedBots[0]
	log.Printf("🔰 Using first available bot: '%s' (%s)", firstBot.Name, firstBot.API.String())
	return firstBot, nil
}

// uploadFileWithSpecificBot uploads file using a specific named bot
func uploadFileWithSpecificBot(namedBots []config.NamedBot, preferredBotName, contentType, filename string, data []byte, destChatId string) (string, string, error) {
	selectedBot, err := getSpecificNamedBot(namedBots, preferredBotName)
	if err != nil {
		return "", "", err
	}

	log.Printf("📤 Uploading file '%s' (%d bytes) using bot '%s'", filename, len(data), selectedBot.Name)

	fileId, err := selectedBot.API.UploadFile(contentType, filename, data, destChatId)
	if err != nil {
		log.Printf("❌ Bot '%s' failed to upload file: %v", selectedBot.Name, err)
		return "", "", fiber.NewError(500, "Failed to upload file with specific bot")
	}

	log.Printf("✅ Upload successful by bot '%s' - FileID: %s", selectedBot.Name, fileId)
	return fileId, selectedBot.Name, nil
}

// downloadFileWithSpecificBot downloads file using a specific named bot
func downloadFileWithSpecificBot(namedBots []config.NamedBot, preferredBotName, fileId string) ([]byte, string, string, error) {
	selectedBot, err := getSpecificNamedBot(namedBots, preferredBotName)
	if err != nil {
		return nil, "", "", err
	}

	log.Printf("📥 Getting file info for '%s' using bot '%s'", fileId, selectedBot.Name)

	// Step 1: Get file path
	filePath, err := selectedBot.API.GetFile(fileId)
	if err != nil {
		log.Printf("❌ Bot '%s' failed to get file info: %v", selectedBot.Name, err)
		return nil, "", "", fiber.NewError(500, "Failed to get file info with specific bot")
	}

	log.Printf("✅ GetFile successful by bot '%s' for FileID: %s", selectedBot.Name, fileId)

	// Step 2: Download file data
	filePathString := selectedBot.API.Explode(filePath)
	log.Printf("📥 Downloading file data using bot '%s'", selectedBot.Name)

	fileData, contentType, err := selectedBot.API.DownloadFile(filePathString)
	if err != nil {
		log.Printf("❌ Bot '%s' failed to download file: %v", selectedBot.Name, err)
		return nil, "", "", fiber.NewError(500, "Failed to download file with specific bot")
	}

	log.Printf("✅ Download successful by bot '%s' - Downloaded %d bytes", selectedBot.Name, len(fileData))
	return fileData, contentType, selectedBot.Name, nil
}