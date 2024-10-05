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
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/pkg/instagram_api"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
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
		//Transport: &http.Transport{
		//	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		//},
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

	ctx.Set("Content-Type", mimeType)
	return ctx.Status(200).Send(bodyRaw)

}

func ZipMultipleFiles(ctx *fiber.Ctx) error {
	bodyBase64 := ctx.Body()
	bodyRaw := make([]byte, 0)
	_, err := base64.StdEncoding.Decode(bodyRaw, bodyBase64)
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

	archiveName := fmt.Sprintf("%x.zip", sha256.Sum256(bodyBase64))
	zipFile, err := os.Create(archiveName)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	defer zipFile.Close()
	defer os.Remove(archiveName)

	zipWriter := zip.NewWriter(zipFile)
	var wg sync.WaitGroup
	var zipLock sync.Mutex
	var errorChan = make(chan error, len(requestData))

	for _, data := range requestData {
		botName := data[0]
		fileID := data[1]

		botAPI := selectBotAPI(ctx, botName)

		wg.Add(1)
		go func(fileID string) {
			defer wg.Done()

			filePath, err := botAPI.GetFile(fileID)
			if err != nil {
				errorChan <- err
				return
			}

			fileData, resContentType, err := botAPI.DownloadFile(botAPI.Explode(filePath))
			if err != nil {
				errorChan <- err
				return
			}

			mimeType := http.DetectContentType(fileData)
			if strings.Contains(mimeType, "text/plain") {
				mimeType = resContentType
			}

			fileReader := bytes.NewReader(fileData)
			fileExtension := strings.Split(mimeType, "/")[1]

			zipLock.Lock()
			zipFileWriter, err := zipWriter.Create(fileID + "." + fileExtension)
			if err != nil {
				errorChan <- err
				zipLock.Unlock()
				return
			}

			if _, err := io.Copy(zipFileWriter, fileReader); err != nil {
				errorChan <- err
				zipLock.Unlock()
				return
			}
			zipLock.Unlock()
		}(fileID)
	}

	wg.Wait()
	close(errorChan)

	for err := range errorChan {
		_ = zipWriter.Close()
		return ctx.Status(fiber.StatusInternalServerError).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	if err := zipWriter.Close(); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	if _, err := zipFile.Seek(0, 0); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	zippedData, err := io.ReadAll(zipFile)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	ctx.Set("Content-Type", "application/zip")
	ctx.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", archiveName))
	return ctx.Send(zippedData)
}
