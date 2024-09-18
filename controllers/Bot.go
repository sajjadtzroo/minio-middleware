package controllers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/pkg/telegram_api"
	"go-uploader/utils"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"
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

	fileData, resContentType, err := botApi.DownloadFile(filePathString)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	err = os.WriteFile("data.txt", fileData, 0755)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: "Cant write file into os",
		})
	}

	mimeType := http.DetectContentType(fileData)
	if strings.Contains(mimeType, "text/plain") {
		mimeType = resContentType
	}

	file := bytes.NewReader(fileData)
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

	botApi := selectBotAPI(ctx, botName)
	contentType := http.DetectContentType(buf.Bytes())

	fileId, err := botApi.UploadFile(contentType, file.Filename, buf.Bytes(), os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
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

	_, params, err := mime.ParseMediaType(res.Header.Get("Content-Disposition"))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}
	filename := params["filename"] // set to "foo.png"

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	mimeType := http.DetectContentType(resBody)

	botApi := selectBotAPI(ctx, botName)
	fileId, err := botApi.UploadFile(mimeType, filename, resBody, os.Getenv("DEST_CHAT_ID"))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"result": true,
		"fileId": fileId,
	})

}
