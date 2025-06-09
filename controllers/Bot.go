package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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

	minioClient := ctx.Locals("minio").(*config.MinIOClients)
	// objectInfo := minioClient.Storage.Conn().ListObjects(ctx.UserContext(), botName, minio.ListObjectsOptions{
	// 	Prefix:    fileId,
	// 	Recursive: true,
	// 	UseV1:     true,
	// })

	// for info := range objectInfo {
	// 	if info.Size > 0 {
	// 		object, err := minioClient.Storage.Conn().GetObject(ctx.UserContext(), botName, info.Key, minio.GetObjectOptions{})
	// 		if err != nil {
	// 			return ctx.Status(500).JSON(models.GenericResponse{
	// 				Result:  false,
	// 				Message: err.Error(),
	// 			})
	// 		}

	// 		data, _ := io.ReadAll(object)
	// 		ctx.Set("Content-Type", http.DetectContentType(data))
	// 		ctx.Set("X-Serve", "Cache")
	// 		_ = object.Close()
	// 		return ctx.Status(200).Send(data)
	// 	}
	// }

	log.Printf("Downloading from Telegram: %s", fileId)
	botApis := selectBotAPI(ctx, botName)
	logBotAPIs(botApis, botName)
	filePath, selectedBotApi, err := raceGetFile(botApis, fileId)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Failed to download from raceGetFile",
		})
	}

	filePathString := selectedBotApi.Explode(filePath.(string))
	fileData, resContentType, err := raceDownloadFile(botApis, filePathString)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Failed to download from raceDownloadFile",
		})
	}
	log.Printf("Downloaded from Telegram: %s", fileId)
	mimeType := http.DetectContentType(fileData)
	if strings.Contains(mimeType, "text/plain") {
		mimeType = resContentType
	}

	file := bytes.NewReader(fileData)
	log.Printf("Uploading to MinIO: %s", fileId)
	_, err = minioClient.Storage.Conn().PutObject(
		ctx.UserContext(),
		botName,
		fileId+"."+strings.Split(mimeType, "/")[1],
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
	log.Printf("Uploaded to MinIO: %s", fileId)
	ctx.Set("X-Serve", "Telegram")
	ctx.Set("Content-Type", mimeType)
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

	botApis := selectBotAPI(ctx, botName)
	contentType := http.DetectContentType(buf.Bytes())

	fileId, err := raceUploadFile(botApis, contentType, file.Filename, buf.Bytes(), os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		log.Printf("Erorr Occured -> %s", err.Error())
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Failed to upload to raceUploadFile",
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"result": true,
		"fileId": fileId,
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

	botApis := selectBotAPI(ctx, botName)
	fileId, err := raceUploadFile(botApis, mimeType, fileName, resBody, os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		log.Printf("Erorr Occured -> %s", err.Error())
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Failed to upload to raceUploadFile",
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"result": true,
		"fileId": fileId,
	})

}
