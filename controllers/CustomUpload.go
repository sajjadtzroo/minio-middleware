package controllers

import (
	"bytes"
	"context"
	"encoding/hex"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/utils"
	"io"
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
	defer src.Close()
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
