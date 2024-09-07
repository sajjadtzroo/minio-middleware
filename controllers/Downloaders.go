package controllers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"io"
)

func DownloadFile(ctx *fiber.Ctx) error {
	path := ctx.Query("path", "")
	if len(path) == 0 {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "path is required",
		})
	}

	bucket := ctx.Query("bucket", "")
	if len(bucket) == 0 {
		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "bucket is required",
		})
	}

	minioClient := ctx.Locals("minio").(*config.MinIOClients)
	object, err := minioClient.Storage.Conn().GetObject(context.Background(), bucket, path, minio.GetObjectOptions{})
	defer object.Close()
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	stat, err := object.Stat()
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	data, err := io.ReadAll(object)
	ctx.Set("Content-Type", stat.ContentType)
	ctx.Set("ETag", stat.ETag)
	return ctx.Status(200).Send(data)
}
