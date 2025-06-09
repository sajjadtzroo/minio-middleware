package controllers

import (
	"go-uploader/config"
	"go-uploader/pkg/telegram_api"
	"log"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type TransferRequest struct {
	FileId           string `json:"fileId"`
	ChatId           string `json:"chatId"`
	BotToken         string `json:"botToken"`
	BotName          string `json:"botName"`
	PreferredBotName string `json:"preferredBotName,omitempty"` // Optional: specific bot name to use
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

	log.Printf("🔄 Starting file transfer: FileID %s from external bot to scope '%s'", req.FileId, req.BotName)

	// Get destination named bots for enhanced logging
	botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
	destNamedBots := botScopeConfig.GetNamedBots(strings.ToLower(req.BotName))
	logNamedBots(destNamedBots, strings.ToLower(req.BotName))

	sourceBotAPI := telegram_api.New(req.BotToken)
	log.Printf("📥 Source bot: %s", sourceBotAPI.String())

	sourceFilePath, err := sourceBotAPI.GetFile(req.FileId)
	if err != nil {
		log.Printf("❌ Failed to get file id: %s -> %v", req.FileId, err.Error())
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to GetFilePath",
		})
	}

	splitSourceFilePath := strings.Split(sourceFilePath, req.BotToken)
	fileData, contentType, err := sourceBotAPI.DownloadFile(splitSourceFilePath[1])
	if err != nil {
		log.Printf("❌ Failed to download file: %s -> %v", sourceFilePath, err.Error())
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to Download requested file from path",
		})
	}

	log.Printf("📥 Downloaded %d bytes from source bot", len(fileData))

	fileName := filepath.Base(sourceFilePath)

	// Log if specific bot was requested
	if req.PreferredBotName != "" {
		log.Printf("🎯 Requested specific destination bot: '%s'", req.PreferredBotName)
	}

	// Use specific bot for upload (defaults to "relic" or first bot)
	finalFileId, usedBotName, err := uploadFileWithSpecificBot(destNamedBots, req.PreferredBotName, contentType, fileName, fileData, req.ChatId)
	if err != nil {
		log.Printf("❌ Failed to upload file: %s -> %v", fileName, err.Error())
		return ctx.Status(500).JSON(fiber.Map{
			"result":  false,
			"message": "Failed to Upload requested file to specific bot",
		})
	}

	log.Printf("✅ Transfer completed: FileID %s transferred to bot '%s' -> New FileID: %s", req.FileId, usedBotName, finalFileId)

	return ctx.Status(200).JSON(fiber.Map{
		"result":        true,
		"fileId":        finalFileId,
		"transferredBy": usedBotName,
	})
}
