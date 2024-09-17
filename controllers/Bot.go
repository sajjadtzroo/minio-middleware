package controllers

import (
	"bytes"
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/pkg/telegram_api"
	"go-uploader/utils"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
)

func selectBotAPI(ctx *fiber.Ctx, botName string) *telegram_api.TelegramAPI {
	switch botName {
	case "instagram":
		return ctx.Locals("BOT_INSTAGRAM").(*telegram_api.TelegramAPI)
	case "telegram":
		return ctx.Locals("BOT_TELEGRAM").(*telegram_api.TelegramAPI)
	case "tracker":
		return ctx.Locals("BOT_TRACKER").(*telegram_api.TelegramAPI)
	case "influencer":
		return ctx.Locals("BOT_INFLUENCER").(*telegram_api.TelegramAPI)
	default:
		return nil
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
	objectInfo := minioClient.Storage.Conn().ListObjects(ctx.UserContext(), botName, minio.ListObjectsOptions{
		Prefix:    fileId,
		Recursive: true,
		UseV1:     true,
	})
	for info := range objectInfo {
		if info.Size > 0 {
			object, err := minioClient.Storage.Conn().GetObject(ctx.UserContext(), botName, info.Key, minio.GetObjectOptions{})
			if err != nil {
				return ctx.Status(500).JSON(models.GenericResponse{
					Result:  false,
					Message: err.Error(),
				})
			}

			data, _ := io.ReadAll(object)
			ctx.Set("Content-Type", http.DetectContentType(data))
			ctx.Set("X-Serve", "Cache")
			_ = object.Close()
			return ctx.Status(200).Send(data)
		}
	}

	botApi := selectBotAPI(ctx, botName)
	filePath, err := botApi.GetFile(fileId)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	filePathString := botApi.Explode(filePath)

	fileData, err := botApi.DownloadFile(filePathString)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	responseFileBody, err := io.ReadAll(fileData)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	mimeType := http.DetectContentType(responseFileBody)

	file := bytes.NewReader(responseFileBody)
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

	ctx.Set("X-Serve", "Telegram")
	ctx.Set("Content-Type", mimeType)
	return ctx.Send(responseFileBody)
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

	botApi := selectBotAPI(ctx, botName)
	contentType := http.DetectContentType(buf.Bytes())

	fileId, err := botApi.UploadFile(contentType, file.Filename, buf.Bytes(), os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	return ctx.Status(200).JSON(models.GenericResponse{
		Result:  true,
		Message: fileId,
	})
}
