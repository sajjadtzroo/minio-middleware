package controllers

import (
	"go-uploader/pkg/telegram_api"
	"log"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type TransferRequest struct {
	FileId   string `json:"fileId"`
	ChatId   string `json:"chatId"`
	BotToken string `json:"botToken"`
	BotName  string `json:"botName"`
}

func TransferFileId(ctx *fiber.Ctx) error {
	var req TransferRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(400).JSON(fiber.Map{
			"result":  false,
			"message": "Invalid request body",
		})
	}

	if req.FileId == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"result":  false,
			"message": "File ID is required",
		})
	}

	if req.ChatId == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"result":  false,
			"message": "chatId is required",
		})
	}

	if req.BotToken == "" {
		return ctx.Status(400).JSON(fiber.Map{
			"result":  false,
			"message": "Bot token is required",
		})
	}

	destBotAPIs := selectBotAPI(ctx, strings.ToLower(req.BotName))
	sourceBotAPI := telegram_api.New(req.BotToken)

	sourceFilePath, err := sourceBotAPI.GetFile(req.FileId)
	if err != nil {
		log.Printf("Failed to get file id: %s\n -> %v", req.FileId, err.Error())
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to GetFilePath",
		})
	}

	splitSourceFilePath := strings.Split(sourceFilePath, req.BotToken)
	fileData, contentType, err := sourceBotAPI.DownloadFile(splitSourceFilePath[1])
	if err != nil {
		log.Printf("Failed to download file: %s\n -> %v", sourceFilePath, err.Error())
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to Download requested file from path",
		})
	}

	fileName := filepath.Base(sourceFilePath)
	finalFileId, err := raceUploadFile(destBotAPIs, contentType, fileName, fileData, req.ChatId)
	if err != nil {
		log.Printf("Failed to upload file: %s\n -> %v", fileName, err.Error())
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to Upload requested file to destBotApi",
		})
	}

	return ctx.Status(200).JSON(fiber.Map{
		"result": true,
		"fileId": finalFileId,
	})
}
