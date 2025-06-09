# Bot Scope Configuration

This document explains how to configure and use bot scopes for racing functionality in the MinIO Middleware project.

## Overview

The bot scope system allows you to configure arrays of Telegram bots for different purposes. When an operation is performed, all bots in the scope race to complete the task, with the first successful response being returned. This provides improved reliability and performance.

## Configuration

### Environment Variables

#### Bot Tokens (Supports Multiple Tokens)
You can provide single or multiple bot tokens for each scope using comma-separated values:

```bash
# Single token per scope
BOT_TELEGRAM=your_telegram_bot_token
BOT_INSTAGRAM=your_instagram_bot_token  
BOT_TRACKER=your_tracker_bot_token
BOT_INFLUENCER=your_influencer_bot_token

# Multiple tokens per scope (comma-separated)
BOT_TELEGRAM=token1,token2,token3
BOT_INSTAGRAM=token1,token2
BOT_TRACKER=token1,token2,token3
BOT_INFLUENCER=token1,token2,token3,token4
```

### Default Scope Configuration

Each scope uses all tokens provided in its environment variable:

- **telegram**: All tokens from `BOT_TELEGRAM` (comma-separated)
- **instagram**: All tokens from `BOT_INSTAGRAM` + telegram fallback if only one token
- **tracker**: All tokens from `BOT_TRACKER` + telegram fallback if only one token  
- **influencer**: All tokens from `BOT_INFLUENCER` + telegram fallback if only one token

**Fallback Logic**: If a scope has only one token, the first telegram bot is added as fallback (except for telegram scope itself).

## Usage in Controllers

The system uses a single `BOT_SCOPE_CONFIG` in middleware that contains all bot configurations:

```go
func selectBotAPI(ctx *fiber.Ctx, botName string) []*telegram_api.TelegramAPI {
    if botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG"); botScopeConfig != nil {
        config := botScopeConfig.(*config.BotScopeConfiguration)
        return config.GetScope(botName)
    }
    return nil
}
```

This approach is simpler and more efficient as it:
- Uses only one configuration object in middleware
- Eliminates redundant `BOTS_*` locals
- Provides centralized access to all bot scopes

## Racing Operations

The system includes three racing functions:

1. **raceGetFile**: Gets file info from multiple bots concurrently
2. **raceDownloadFile**: Downloads files from multiple bots concurrently  
3. **raceUploadFile**: Uploads files to multiple bots concurrently

All racing functions return the result from the first bot that succeeds.

## Advanced Configuration

Since all bot configurations are centralized in `BOT_SCOPE_CONFIG`, you can easily extend functionality:

### Adding Bots to Scopes Dynamically
```go
botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
newBot := telegram_api.New("new_bot_token")
botScopeConfig.AddBotToScope("telegram", newBot)
```

### Creating Custom Scopes
```go
botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
customTokens := []string{"token1", "token2", "token3"}
botScopeConfig.CreateCustomScope("custom_scope", customTokens)

// Then use it
customBots := botScopeConfig.GetScope("custom_scope")
```

### Accessing All Available Scopes
```go
botScopeConfig := ctx.Locals("BOT_SCOPE_CONFIG").(*config.BotScopeConfiguration)
// Access the entire scopes map for advanced operations
allScopes := botScopeConfig.Scopes
```

## Benefits

1. **Improved Reliability**: If one bot fails, others automatically take over
2. **Better Performance**: Multiple bots race, fastest response wins
3. **Load Distribution**: Requests are distributed across multiple bot instances
4. **Flexible Configuration**: Easy to add/remove bots from scopes
5. **Centralized Management**: Single `BOT_SCOPE_CONFIG` contains all bot configurations
6. **Clean Architecture**: Eliminates redundant middleware locals
7. **Easy Extensibility**: Simple to add custom scopes and advanced configurations

## Example .env Configuration

```bash
# Single bot per scope
BOT_TELEGRAM=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
BOT_INSTAGRAM=9876543210:ZYXwvuTSRQponMLKjihGFEdcba
BOT_TRACKER=1111111111:AAAAaaaBBBbbbCCCcccDDDdddEEE
BOT_INFLUENCER=2222222222:FFFFfffGGGgggHHHhhhIIIiiiJJJ
```

```bash
# Multiple bots per scope for better racing performance
BOT_TELEGRAM=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz,3333333333:KKKkkkLLLlllMMM,4444444444:NNNnnnOOOooo
BOT_INSTAGRAM=9876543210:ZYXwvuTSRQponMLKjihGFEdcba,5555555555:PPPpppQQQqqq
BOT_TRACKER=1111111111:AAAAaaaBBBbbbCCCcccDDDdddEEE,6666666666:RRRrrrSSS
BOT_INFLUENCER=2222222222:FFFFfffGGGgggHHHhhhIIIiiiJJJ,7777777777:TTTtttUUU,8888888888:VVVvvvWWW,9999999999:XXXxxxYYY
```

This configuration provides multiple bots per scope for optimal racing performance and reliability. 