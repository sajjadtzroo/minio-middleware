# Uploader Service (GO)

It's simple uploader service written in golang.

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

