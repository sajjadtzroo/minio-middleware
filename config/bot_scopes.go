package config

import (
	"fmt"
	"go-uploader/pkg/telegram_api"
	"os"
	"strings"
)

// NamedBot represents a bot with its name and API instance
type NamedBot struct {
	Name string
	API  *telegram_api.TelegramAPI
}

// BotScope represents a collection of named bots for a specific scope
type BotScope struct {
	Name      string
	NamedBots []NamedBot
}

// BotScopeConfiguration manages all bot scopes
type BotScopeConfiguration struct {
	Scopes map[string][]NamedBot
}

// NewBotScopeConfiguration creates and configures bot scopes
func NewBotScopeConfiguration() *BotScopeConfiguration {
	// Parse bot tokens from environment variables (supports comma-separated values)
	telegramBots := parseNamedBotsFromEnv("BOT_TELEGRAM", "telegram")
	instagramBots := parseNamedBotsFromEnv("BOT_INSTAGRAM", "instagram")
	trackerBots := parseNamedBotsFromEnv("BOT_TRACKER", "tracker")
	influencerBots := parseNamedBotsFromEnv("BOT_INFLUENCER", "influencer")

	// Configure scopes with named bots
	scopes := map[string][]NamedBot{
		"telegram":   telegramBots,
		"instagram":  instagramBots,
		"tracker":    trackerBots,
		"influencer": influencerBots,
	}

	return &BotScopeConfiguration{
		Scopes: scopes,
	}
}

// parseNamedBotsFromEnv parses comma-separated bot tokens with names from environment variable
// Format: name1<token1>,name2<token2>,name3<token3>
// If no name is provided (just token), defaults to "relic" for first bot and scope_N for others
func parseNamedBotsFromEnv(envKey, scopeName string) []NamedBot {
	envValue := os.Getenv(envKey)
	if envValue == "" {
		return []NamedBot{}
	}

	tokens := strings.Split(envValue, ",")
	var namedBots []NamedBot

	for i, entry := range tokens {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			var botName, botToken string

			// Check if entry contains name<token> format
			if strings.Contains(entry, "<") && strings.Contains(entry, ">") {
				// Parse name<token> format
				startIdx := strings.Index(entry, "<")
				endIdx := strings.LastIndex(entry, ">")

				if startIdx > 0 && endIdx > startIdx {
					botName = strings.TrimSpace(entry[:startIdx])
					botToken = strings.TrimSpace(entry[startIdx+1 : endIdx])
				} else {
					// Invalid format, skip this entry
					continue
				}
			} else {
				// No name provided, use default naming
				botToken = entry
				if i == 0 {
					botName = "relic" // First bot is always named "relic"
				} else {
					botName = fmt.Sprintf("%s_%d", scopeName, i+1)
				}
			}

			if botToken != "" && botName != "" {
				bot := telegram_api.New(botToken)
				namedBots = append(namedBots, NamedBot{
					Name: botName,
					API:  bot,
				})
			}
		}
	}

	return namedBots
}

// GetBots returns the bot APIs for a given scope (for backward compatibility)
func (bsc *BotScopeConfiguration) GetBots(scope string) []*telegram_api.TelegramAPI {
	if namedBots, exists := bsc.Scopes[scope]; exists && len(namedBots) > 0 {
		bots := make([]*telegram_api.TelegramAPI, len(namedBots))
		for i, namedBot := range namedBots {
			bots[i] = namedBot.API
		}
		return bots
	}
	return []*telegram_api.TelegramAPI{}
}

// GetNamedBots returns the named bots for a given scope
func (bsc *BotScopeConfiguration) GetNamedBots(scope string) []NamedBot {
	if namedBots, exists := bsc.Scopes[scope]; exists {
		return namedBots
	}
	return []NamedBot{}
}

// GetScope returns the bot array for a specific scope (for backward compatibility)
func (bsc *BotScopeConfiguration) GetScope(scopeName string) []*telegram_api.TelegramAPI {
	return bsc.GetBots(scopeName)
}

// AddBotToScope adds a named bot to a specific scope
func (bsc *BotScopeConfiguration) AddBotToScope(scopeName string, botName string, bot *telegram_api.TelegramAPI) {
	if bsc.Scopes[scopeName] == nil {
		bsc.Scopes[scopeName] = []NamedBot{}
	}
	namedBot := NamedBot{Name: botName, API: bot}
	bsc.Scopes[scopeName] = append(bsc.Scopes[scopeName], namedBot)
}

// CreateCustomScope creates a new scope with specified tokens and names
// Supports both name<token> format and plain tokens
func (bsc *BotScopeConfiguration) CreateCustomScope(scopeName string, tokens []string) {
	var namedBots []NamedBot
	for i, entry := range tokens {
		entry = strings.TrimSpace(entry)
		if entry != "" {
			var botName, botToken string

			// Check if entry contains name<token> format
			if strings.Contains(entry, "<") && strings.Contains(entry, ">") {
				// Parse name<token> format
				startIdx := strings.Index(entry, "<")
				endIdx := strings.LastIndex(entry, ">")

				if startIdx > 0 && endIdx > startIdx {
					botName = strings.TrimSpace(entry[:startIdx])
					botToken = strings.TrimSpace(entry[startIdx+1 : endIdx])
				} else {
					// Invalid format, skip this entry
					continue
				}
			} else {
				// No name provided, use default naming
				botToken = entry
				if i == 0 {
					botName = "relic" // First bot is always named "relic"
				} else {
					botName = fmt.Sprintf("%s_%d", scopeName, i+1)
				}
			}

			if botToken != "" && botName != "" {
				bot := telegram_api.New(botToken)
				namedBots = append(namedBots, NamedBot{
					Name: botName,
					API:  bot,
				})
			}
		}
	}
	bsc.Scopes[scopeName] = namedBots
}

// GetAllScopes returns all available scope names
func (bsc *BotScopeConfiguration) GetAllScopes() []string {
	scopes := make([]string, 0, len(bsc.Scopes))
	for scope, namedBots := range bsc.Scopes {
		if len(namedBots) > 0 {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// GetAllScopeDetails returns detailed information about all scopes
func (bsc *BotScopeConfiguration) GetAllScopeDetails() map[string][]string {
	scopeDetails := make(map[string][]string)
	for scope, namedBots := range bsc.Scopes {
		if len(namedBots) > 0 {
			botNames := make([]string, len(namedBots))
			for i, namedBot := range namedBots {
				botNames[i] = namedBot.Name
			}
			scopeDetails[scope] = botNames
		}
	}
	return scopeDetails
}
