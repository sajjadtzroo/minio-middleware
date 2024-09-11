package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/pkg/instagram_api"
	"io"
	"net/http"
	"slices"
	"strings"
)

func DownloadFile(ctx *fiber.Ctx) error {
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
	objectInfo := minioClient.Storage.Conn().ListObjects(context.Background(), bucket, minio.ListObjectsOptions{
		Prefix:    path,
		Recursive: true,
		UseV1:     true,
	})

	for info := range objectInfo {
		if info.Size > 0 {
			if strings.Split(info.Key, ".")[0] == path {
				object, err := minioClient.Storage.Conn().GetObject(context.Background(), bucket, info.Key, minio.GetObjectOptions{})
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

	objectInfo := minioClient.Storage.Conn().ListObjects(context.Background(), bucketName, minio.ListObjectsOptions{
		Prefix:    pk,
		Recursive: true,
		UseV1:     true,
	})

	for info := range objectInfo {
		if info.Size > 0 {
			if strings.Split(info.Key, ".")[0] == pk {
				object, err := minioClient.Storage.Conn().GetObject(context.Background(), bucketName, info.Key, minio.GetObjectOptions{})
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

	if media == "telegram" {
		var reqUrl string
		if strings.HasPrefix(userName, "@") {
			reqUrl = "https://tgobserver.darkube.app/getChannelInfo?channel=" + userName
		} else {
			reqUrl = "https://tgobserver.darkube.app/getChannelInfo?channel_link=" + userName
		}

		res, err := http.Get(reqUrl)
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

		photoRes, err := http.Get("https://tgobserver.darkube.app" + telegramProfile.ProfilePhoto)
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
		file := bytes.NewReader(responseFileBody)
		_, err = minioClient.Storage.Conn().PutObject(
			context.Background(),
			bucketName,
			pk+".jpg",
			file,
			file.Size(),
			minio.PutObjectOptions{},
		)

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
	req, err := http.NewRequest("GET", snitchConfig.Url, nil)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	req.Header.Set("X-Proxy-To", profilePicUrl)
	picRes, err := http.DefaultClient.Do(req)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	defer picRes.Body.Close()
	bodyRaw, _ := io.ReadAll(picRes.Body)

	if picRes.StatusCode != 200 {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: string(bodyRaw),
		})
	}

	mimeType := http.DetectContentType(bodyRaw)
	file := bytes.NewReader(bodyRaw)
	_, err = minioClient.Storage.Conn().PutObject(
		context.Background(),
		bucketName,
		pk+".jpg",
		file,
		file.Size(),
		minio.PutObjectOptions{},
	)

	ctx.Set("Content-Type", mimeType)
	return ctx.Status(200).Send(bodyRaw)

}
