package controllers

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/minio/minio-go/v7"
	"go-uploader/config"
	"go-uploader/models"
	"go-uploader/utils"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
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

	src, err := file.Open()
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}
	defer func(src multipart.File) {
		_ = src.Close()
	}(src)

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, src); err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
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

	minioClient, err := getLocal[*config.MinIOClients](ctx, "minio")
	if err != nil {
		return err
	}
	filename := utils.CreateFilePath(hex.EncodeToString(fileId), utils.ImageFileTypes[file.Header.Get("Content-Type")])
	_, err = minioClient.Storage.Conn().PutObject(
		ctx.UserContext(),
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

	if err := validateExternalURL(body.Link); err != nil {
		return ctx.Status(400).JSON(models.GenericResponse{Result: false, Message: err.Error()})
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

	req, err := http.NewRequestWithContext(ctx.UserContext(), "GET", requestURI.String(), nil)
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	client := &http.Client{
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

	resBody, err := io.ReadAll(io.LimitReader(res.Body, maxDownloadSize))
	if err != nil {
		return ctx.Status(500).JSON(models.GenericResponse{
			Result:  false,
			Message: err.Error(),
		})
	}

	file := bytes.NewReader(resBody)
	mimeType := http.DetectContentType(resBody)
	fileExtension, err := utils.GetExtensionFromMimeType(mimeType)
	if err != nil || len(fileExtension) == 0 {
		fileExtension = []string{".bin"}
	}

	minioClient, err := getLocal[*config.MinIOClients](ctx, "minio")
	if err != nil {
		return err
	}
	_, err = minioClient.Storage.Conn().PutObject(
		ctx.UserContext(),
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
