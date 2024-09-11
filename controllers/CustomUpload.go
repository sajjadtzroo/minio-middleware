package controllers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/utils"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

func UploadFile(ctx *fiber.Ctx) error {
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
	if file.Size > utils.MaxImageSize || file.Size < utils.MinImageSize {
		return ctx.Status(422).JSON(models.GenericResponse{
			Result:  false,
			Message: "File Size Error",
		})
	}

	src, err := file.Open()
	defer func(src multipart.File) {
		_ = src.Close()
	}(src)

	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, src); err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	if !utils.ImageAllowedFormats[file.Header.Get("Content-Type")] {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "Image Format Not Allowed",
		})
	}

	bucketName := ctx.Params("bucketName", "")
	if len(bucketName) == 0 {
		return ctx.Status(412).JSON(models.GenericResponse{
			Result:  false,
			Message: "bucketName not set",
		})
	}

	fileId, err := utils.CreateFileID(buf.Bytes())
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	minioClient := ctx.Locals("minio").(*config.MinIOClients)
	filename := utils.CreateFilePath(hex.EncodeToString(fileId), utils.ImageFileTypes[file.Header.Get("Content-Type")])
	_, err = minioClient.Storage.Conn().PutObject(
		context.Background(),
		bucketName,
		filename,
		buf,
		file.Size,
		minio.PutObjectOptions{},
	)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	return ctx.Status(200).JSON(models.UploadedResponse{
		Result: true,
		FileId: hex.EncodeToString(fileId),
	})
}

func DownloadFromLinkAndUpload(ctx *fiber.Ctx) error {
	bodyRaw := ctx.Body()
	var body models.DownLoadFromLinkRequest
	if err := json.Unmarshal(bodyRaw, &body); err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	requestURI, err := url.ParseRequestURI(body.Link)
	if err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	if !slices.Contains(utils.ValidBuckets, body.Bucket) {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "Bucket Not Found",
		})
	}

	if len(body.FileName) == 0 {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "File Name not set",
		})
	}

	if len(strings.Split(body.FileName, ".")) > 1 {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "File name shouldn't have extension",
		})
	}

	req, err := http.NewRequest("GET", requestURI.String(), nil)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	file := bytes.NewReader(resBody)
	mimeType := res.Header.Get("Content-Type")
	fileExtension, err := utils.GetExtensionFromMimeType(mimeType)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	log.Printf("File Extension: %v", fileExtension)

	minioClient := ctx.Locals("minio").(*config.MinIOClients)
	_, err = minioClient.Storage.Conn().PutObject(
		context.Background(),
		body.Bucket,
		body.FileName+fileExtension[0],
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

	return ctx.Status(200).JSON(models.GenericResponse{
		Result:  true,
		Message: "Upload Success",
	})
}
