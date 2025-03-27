package controllers

import (
	"go-uploader/pkg/telegram_api"
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

	destBotAPI := selectBotAPI(ctx, strings.ToLower(req.BotName))
	sourceBotAPI := telegram_api.New(req.BotToken)

	sourceFilePath, err := sourceBotAPI.GetFile(req.FileId)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to GetFilePath",
		})
	}

	fileData, contentType, err := sourceBotAPI.DownloadFile(sourceFilePath)
	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to Download requested file from path",
		})
	}

	fileName := filepath.Base(sourceFilePath)

	finalFileId, err := destBotAPI.UploadFile(contentType, fileName, fileData, req.ChatId)
	if err != nil {
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
