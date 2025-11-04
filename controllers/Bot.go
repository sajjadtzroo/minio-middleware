package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/pkg/telegram_api"
	"go-uploader/utils"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
)

func selectBotAPI(ctx *fiber.Ctx, botName string) []*telegram_api.TelegramAPI {
	if botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG"); botScopeConfig != nil {
		config := botScopeConfig.(*config.BotScopeConfiguration)
		return config.GetScope(botName)
	}
	return nil
}

// logBotAPIs logs bot API information in a safe and readable format
func logBotAPIs(botApis []*telegram_api.TelegramAPI, scope string) {
	if len(botApis) == 0 {
		log.Printf("No Bot APIs found for scope: %s", scope)
		return
	}

	log.Printf("Selected %d Bot APIs for scope '%s':", len(botApis), scope)
	for i, bot := range botApis {
		log.Printf("  [%d] %s", i+1, bot.String())
	}
}

// logNamedBots logs named bot information in a safe and readable format
func logNamedBots(namedBots []config.NamedBot, scope string) {
	if len(namedBots) == 0 {
		log.Printf("No Named Bots found for scope: %s", scope)
		return
	}

	log.Printf("Selected %d Named Bots for scope '%s':", len(namedBots), scope)
	for i, namedBot := range namedBots {
		log.Printf("  [%d] %s (%s)", i+1, namedBot.Name, namedBot.API.String())
	}
}

func DownloadFromTelegram(ctx *fiber.Ctx) error {
	ctx.SetUserContext(context.Background())

	botName := ctx.Params("botName", "")
	if !slices.Contains(utils.ValidBuckets, botName) {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "bot name is not valid",
		})
	}

	fileId := ctx.Params("fileId")
	if len(fileId) == 0 {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "file id is empty",
		})
	}

	// Get optional specific bot name from URL parameter or query param
	specificBotFromURL := ctx.Params("specificBot", "")
	specificBotFromQuery := ctx.Query("bot", "")

	minioClient := ctx.Locals("minio").(*config.MinIOClients)

	// ✅ IMPORTANT: Check MinIO cache first - HUGE performance improvement!
	log.Printf("🔍 Checking MinIO cache for FileID: %s in bucket: %s", fileId, botName)

	// Use short timeout for cache check
	cacheCtx, cancelCache := context.WithTimeout(ctx.UserContext(), 5*time.Second)
	defer cancelCache()

	objectInfo := minioClient.Storage.Conn().ListObjects(cacheCtx, botName, minio.ListObjectsOptions{
		Prefix:    fileId,
		Recursive: true,
		UseV1:     true,
	})

	for info := range objectInfo {
		if info.Size > 0 {
			// File found in cache!
			log.Printf("✅ Cache HIT for FileID: %s (Size: %d bytes, Key: %s)", fileId, info.Size, info.Key)

			getCtx, cancelGet := context.WithTimeout(ctx.UserContext(), 10*time.Second)
			defer cancelGet()

			object, err := minioClient.Storage.Conn().GetObject(getCtx, botName, info.Key, minio.GetObjectOptions{})
			if err != nil {
				log.Printf("❌ Failed to get cached object: %v", err)
				break // Continue to download from Telegram
			}

			data, err := io.ReadAll(object)
			_ = object.Close()

			if err != nil {
				log.Printf("❌ Failed to read cached object: %v", err)
				break // Continue to download from Telegram
			}

			// ✅ Return from cache - MUCH faster!
			ctx.Set("Content-Type", http.DetectContentType(data))
			ctx.Set("X-Serve", "Cache")
			ctx.Set("X-Cache", "HIT")
			ctx.Set("Cache-Control", "public, max-age=86400") // 24 hours browser cache
			ctx.Set("X-Cache-Key", info.Key)
			log.Printf("🚀 Serving from cache: %s (%d bytes)", fileId, len(data))
			return ctx.Status(200).Send(data)
		}
	}

	// ❌ Not in cache, download from Telegram
	log.Printf("❌ Cache MISS for FileID: %s - Downloading from Telegram", fileId)

	// Get named bots for bot selection
	botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
	namedBots := botScopeConfig.GetNamedBots(botName)
	logNamedBots(namedBots, botName)

	// Determine preferred bot name (priority: URL param > query param > racing mode)
	preferredBotName := ""
	useRacing := true

	// First check URL parameter (highest priority)
	if specificBotFromURL != "" {
		preferredBotName = specificBotFromURL
		useRacing = false
		log.Printf("🎯 Requested specific bot from URL: '%s'", preferredBotName)
	} else if specificBotFromQuery != "" {
		// Then check query parameter
		preferredBotName = specificBotFromQuery
		useRacing = false
		log.Printf("🎯 Requested specific bot from query: '%s'", preferredBotName)
	} else {
		log.Printf("🏁 No specific bot requested, using racing mode")
	}

	var fileData []byte
	var resContentType string
	var usedBotName string
	var downloadBotName string
	var err error

	if useRacing {
		// Use optimized racing mode with timeouts
		filePath, selectedBotApi, winningBotName, err := raceGetFileWithNamesOptimized(namedBots, fileId)
		if err != nil {
			log.Printf("❌ raceGetFileWithNamesOptimized failed: %v", err)
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: "Failed to get file info from Telegram",
			})
		}

		filePathString := selectedBotApi.Explode(filePath.(string))

		fileData, resContentType, downloadBotName, err = raceDownloadFileWithNamesOptimized(namedBots, filePathString)
		if err != nil {
			log.Printf("❌ raceDownloadFileWithNamesOptimized failed: %v", err)
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: "Failed to download from Telegram",
			})
		}

		usedBotName = fmt.Sprintf("GetFile:%s|Download:%s", winningBotName, downloadBotName)
		log.Printf("✅ Complete download chain: GetFile by '%s' → DownloadFile by '%s' for FileID: %s", winningBotName, downloadBotName, fileId)
	} else {
		// Use specific bot
		fileData, resContentType, usedBotName, err = downloadFileWithSpecificBot(namedBots, preferredBotName, fileId)
		if err != nil {
			log.Printf("❌ downloadFileWithSpecificBot failed: %v", err)
			return ctx.Status(500).JSON(models.GenericResponse{
				Result:  false,
				Message: "Failed to download with specific bot",
			})
		}
	}

	mimeType := http.DetectContentType(fileData)
	if strings.Contains(mimeType, "text/plain") && resContentType != "" {
		mimeType = resContentType
	}

	// ✅ Upload to MinIO for future caching - in background
	go func() {
		uploadCtx, cancelUpload := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancelUpload()

		file := bytes.NewReader(fileData)
		extension := "bin"
		if parts := strings.Split(mimeType, "/"); len(parts) == 2 {
			extension = parts[1]
		}
		fileName := fileId + "." + extension

		log.Printf("📤 Uploading to MinIO cache: %s (size: %d bytes)", fileName, file.Size())

		_, err := minioClient.Storage.Conn().PutObject(
			uploadCtx,
			botName,
			fileName,
			file,
			file.Size(),
			minio.PutObjectOptions{
				ContentType: mimeType,
			},
		)

		if err != nil {
			log.Printf("❌ Failed to cache in MinIO: %v", err)
		} else {
			log.Printf("✅ Successfully cached in MinIO: %s", fileName)
		}
	}()

	// Return response immediately
	ctx.Set("X-Serve", "Telegram")
	ctx.Set("X-Cache", "MISS")
	ctx.Set("Content-Type", mimeType)
	ctx.Set("X-Downloaded-By", usedBotName)
	ctx.Set("Cache-Control", "public, max-age=86400") // 24 hours browser cache
	log.Printf("🚀 Serving from Telegram: %s (%d bytes)", fileId, len(fileData))
	return ctx.Send(fileData)
}

func UploadToTelegram(ctx *fiber.Ctx) error {
	ctx.SetUserContext(context.Background())

	botName := ctx.Params("botName", "")
	if !slices.Contains(utils.ValidBuckets, botName) {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "bot name is not valid",
		})
	}

	// Get optional specific bot name from URL parameter (for backward compatibility)
	specificBotFromURL := ctx.Params("specificBot", "")

	form, err := ctx.MultipartForm()
	if err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	if len(form.File["file"]) == 0 {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "File not uploaded",
		})
	}

	file := form.File["file"][0]

	buf, err := utils.OpenFile(file)
	if err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	// Get named bots for specific bot selection
	botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
	namedBots := botScopeConfig.GetNamedBots(botName)
	logNamedBots(namedBots, botName)

	// Determine preferred bot name (priority: URL param > form field > default to "relic")
	preferredBotName := ""

	// First check URL parameter (highest priority)
	if specificBotFromURL != "" {
		preferredBotName = specificBotFromURL
		log.Printf("🎯 Requested specific bot from URL: '%s'", preferredBotName)
	} else if form.Value["botName"] != nil && len(form.Value["botName"]) > 0 {
		// Then check form field
		preferredBotName = form.Value["botName"][0]
		log.Printf("🎯 Requested specific bot from form: '%s'", preferredBotName)
	}
	// If neither provided, preferredBotName stays empty and defaults to "relic"

	contentType := http.DetectContentType(buf.Bytes())

	// Use specific bot for upload (defaults to "relic" or first bot if preferredBotName is empty)
	fileId, usedBotName, err := uploadFileWithSpecificBot(namedBots, preferredBotName, contentType, file.Filename, buf.Bytes(), os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		log.Printf("Error Occurred -> %s", err.Error())
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Failed to upload with specific bot",
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"result":     true,
		"fileId":     fileId,
		"uploadedBy": usedBotName,
	})
}

func UploadToTelegramViaLink(ctx *fiber.Ctx) error {
	botName := ctx.Params("botName", "")
	if !slices.Contains(utils.ValidBuckets, botName) {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "Bucket Not Found",
		})
	}

	bodyRaw := ctx.Body()
	var body map[string]string
	if err := json.Unmarshal(bodyRaw, &body); err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	if _, ok := body["link"]; !ok {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "link is empty",
		})
	}

	requestURI, err := url.ParseRequestURI(body["link"])
	if err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "link is invalid",
		})
	}

	req, err := http.NewRequestWithContext(ctx.UserContext(), "GET", requestURI.String(), nil)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        10,
			MaxConnsPerHost:     10,
			MaxIdleConnsPerHost: 10,
		},
		Timeout: 60 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	splitUrl := strings.Split(body["link"], "/")
	fileName := splitUrl[len(splitUrl)-1]

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	mimeType := http.DetectContentType(resBody)

	// Get named bots for specific bot selection
	botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
	namedBots := botScopeConfig.GetNamedBots(botName)
	logNamedBots(namedBots, botName)

	// Check if specific bot name is provided in request body
	preferredBotName := ""
	if botNameFromBody, exists := body["botName"]; exists && botNameFromBody != "" {
		preferredBotName = botNameFromBody
		log.Printf("🎯 Requested specific bot from body: '%s'", preferredBotName)
	}

	// Use specific bot for upload (defaults to "relic" or first bot)
	fileId, usedBotName, err := uploadFileWithSpecificBot(namedBots, preferredBotName, mimeType, fileName, resBody, os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		log.Printf("Error Occurred -> %s", err.Error())
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Failed to upload with specific bot",
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"result":     true,
		"fileId":     fileId,
		"uploadedBy": usedBotName,
	})

}

// ListBotScopes returns all available bot scopes with their named bots
func ListBotScopes(ctx *fiber.Ctx) error {
	botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG")
	if botScopeConfig == nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Bot scope configuration not found",
		})
	}

	config := botScopeConfig.(*config.BotScopeConfiguration)
	scopeDetails := config.GetAllScopeDetails()

	return ctx.Status(200).JSON(fiber.Map{
		"result": true,
		"scopes": scopeDetails,
	})
}