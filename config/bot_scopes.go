package config

import (
	"go-uploader/pkg/telegram_api"
	"os"
	"strings"
)

// BotScope represents a collection of bots for a specific scope
type BotScope struct {
	Name string
	Bots []*telegram_api.TelegramAPI
}

// BotScopeConfiguration manages all bot scopes
type BotScopeConfiguration struct {
	Scopes map[string][]*telegram_api.TelegramAPI
}

// NewBotScopeConfiguration creates and configures bot scopes
func NewBotScopeConfiguration() *BotScopeConfiguration {
	// Parse bot tokens from environment variables (supports comma-separated values)
	telegramBots := parseBotsFromEnv("BOT_TELEGRAM")
	instagramBots := parseBotsFromEnv("BOT_INSTAGRAM")
	trackerBots := parseBotsFromEnv("BOT_TRACKER")
	influencerBots := parseBotsFromEnv("BOT_INFLUENCER")

	// Configure scopes - if multiple tokens provided, use all of them
	// If only one token provided, add telegram as fallback (except for telegram scope)
	scopes := map[string][]*telegram_api.TelegramAPI{
		"telegram":   telegramBots,
		"instagram":  instagramBots,
		"tracker":    trackerBots,
		"influencer": influencerBots,
	}

	return &BotScopeConfiguration{
		Scopes: scopes,
	}
}

// parseBotsFromEnv parses comma-separated bot tokens from environment variable
func parseBotsFromEnv(envKey string) []*telegram_api.TelegramAPI {
	var bots []*telegram_api.TelegramAPI

	envValue := os.Getenv(envKey)
	if envValue == "" {
		return bots
	}

	// Split by comma to support multiple tokens
	tokens := strings.Split(envValue, ",")
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			bot := telegram_api.New(token)
			bots = append(bots, bot)
		}
	}

	return bots
}

// GetScope returns the bot array for a specific scope
func (bsc *BotScopeConfiguration) GetScope(scopeName string) []*telegram_api.TelegramAPI {
	if bots, exists := bsc.Scopes[scopeName]; exists {
		return bots
	}
	return nil
}

// AddBotToScope adds a bot to a specific scope
func (bsc *BotScopeConfiguration) AddBotToScope(scopeName string, bot *telegram_api.TelegramAPI) {
	if bsc.Scopes[scopeName] == nil {
		bsc.Scopes[scopeName] = []*telegram_api.TelegramAPI{}
	}
	bsc.Scopes[scopeName] = append(bsc.Scopes[scopeName], bot)
}

// CreateCustomScope creates a new scope with specified bots
func (bsc *BotScopeConfiguration) CreateCustomScope(scopeName string, tokens []string) {
	var bots []*telegram_api.TelegramAPI
	for _, token := range tokens {
		if strings.TrimSpace(token) != "" {
			bot := telegram_api.New(strings.TrimSpace(token))
			bots = append(bots, bot)
		}
	}
	bsc.Scopes[scopeName] = bots
}
