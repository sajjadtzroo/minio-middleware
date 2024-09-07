package controllers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"io"
	"log"
	"slices"
	"strings"
)

func DownloadFile(ctx *fiber.Ctx) error {
	reqPath := strings.Split(ctx.Path(), "/")
	if len(reqPath) < 3 {
		if reqPath[1] == "favicon.ico" {
			return ctx.SendStatus(404)
		}

		return ctx.Status(400).JSON(models.GenericResponse{
			Result:  false,
			Message: "Invalid path",
		})
	}

	bucket := reqPath[1]
	reqPath = slices.Delete(reqPath, 0, 2)
	path := strings.Join(reqPath, "/")

	log.Printf("choosed %s bucket and downloading file %s", bucket, path)

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
