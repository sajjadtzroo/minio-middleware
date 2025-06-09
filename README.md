# Uploader Service (GO)

It's simple uploader service written in golang with advanced bot racing capabilities.

## 🚀 Features

- **Bot Racing System**: Multiple Telegram bots race for fastest response
- **Flexible Bot Scopes**: Configure multiple bots per scope for improved reliability
- **File Upload/Download**: Support for direct storage and Telegram bot storage
- **Profile Picture Fetching**: Instagram and Telegram profile picture downloads
- **JWT Authentication**: Secure endpoint protection
- **ZIP Archive Creation**: Batch file operations

## 🔧 Bot Configuration

Configure bot tokens in your `.env` file. Supports multiple tokens per scope:

```bash
# Single bot per scope
BOT_TELEGRAM=your_telegram_bot_token
BOT_INSTAGRAM=your_instagram_bot_token
BOT_TRACKER=your_tracker_bot_token
BOT_INFLUENCER=your_influencer_bot_token

# Multiple bots per scope (comma-separated for racing)
BOT_INFLUENCER=token1,token2,token3,token4
```

See [README_BOT_SCOPES.md](README_BOT_SCOPES.md) for detailed configuration information.

# `POST` /upload/telegram/:botName

Use This Route to upload any file to selected telegram bot and return telegram file_id on `fileId`

# `GET` /profile/:media/:pk/:userName

Getting Social Media image from this route.\
If Telegram Public channel use `@` at first of them

# `POST` /instant/link

Upload a URL to a bucket from a link\
`FileName` as fileName\
`Bucket` as bucketName

# `GET` /instant/:botName/:fileId

Get a File From Bot Bucket Without extension needing - If not exists, it will download it from telegram

# `POST` /direct/:bucketName

Upload a file on specific Bucket

# `GET` /direct/:bucketName/:path

Get a file on specific Bucket

