package telegram_api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)
const BaseUrl = "http://94.130.99.214"
// const BaseUrl = "https://api.telegram.org"
const ContentType = "application/json"

type TelegramAPI struct {
	client *http.Client
	token  string
}

func New(token string) *TelegramAPI {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 300 * time.Second,
	}

	api := TelegramAPI{
		client,
		token,
	}

	return &api
}

// String returns a safe string representation for logging
func (h *TelegramAPI) String() string {
	if len(h.token) > 10 {
		return fmt.Sprintf("TelegramAPI{token: %s...}", h.token[:10])
	}
	return "TelegramAPI{token: ***}"
}

type GetFileResponse struct {
	Ok     bool   `json:"ok"`
	Result struct {
		FilePath string `json:"file_path"`
		FileSize int64  `json:"file_size"`
		FileId   string `json:"file_id"`
	} `json:"result"`
	Description string `json:"description,omitempty"`
}

func (h *TelegramAPI) GetFile(fileId string) (string, error) {
	bodyRaw := map[string]string{
		"file_id": fileId,
	}
	reqURL := BaseUrl + "/bot" + h.token + "/getFile"
	body, err := json.Marshal(bodyRaw)
	if err != nil {
		return "", err
	}

	response, err := h.client.Post(reqURL, ContentType, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	defer response.Body.Close()
	resBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		return "", errors.New("telegram failed " + string(resBody))
	}

	var result GetFileResponse
	errJson := json.Unmarshal(resBody, &result)
	if errJson != nil {
		return "", errJson
	}

	log.Printf("📁 GetFile successful: %s (size: %d bytes)", result.Result.FilePath, result.Result.FileSize)
	return result.Result.FilePath, nil
}

func (h *TelegramAPI) DownloadFile(filePath string) ([]byte, string, error) {
	// تمیز کردن مسیر
	cleanPath := strings.TrimPrefix(filePath, "/")

	reqURL := BaseUrl + "/file/bot" + h.token + "/" + cleanPath
	log.Printf("📥 Downloading from: %s", reqURL)

	response, err := h.client.Get(reqURL)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}

	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	if response.StatusCode != 200 {
		// اگه از پروکسی 404 گرفت، سعی کن از API اصلی
		if response.StatusCode == 404 {
			log.Printf("⚠️ Proxy returned 404, trying official Telegram API...")

			officialURL := "https://api.telegram.org/file/bot" + h.token + "/" + cleanPath
			response2, err := h.client.Get(officialURL)
			if err != nil {
				return nil, "", fmt.Errorf("official API also failed: %w", err)
			}
			defer response2.Body.Close()

			resBody2, err := io.ReadAll(response2.Body)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read from official API: %w", err)
			}

			if response2.StatusCode == 200 {
				log.Printf("✅ Downloaded from official API successfully")
				return resBody2, response2.Header.Get("Content-Type"), nil
			}

			return nil, "", fmt.Errorf("both proxy and official API failed (status %d)", response2.StatusCode)
		}

		return nil, "", fmt.Errorf("download failed (status %d): %s", response.StatusCode, string(resBody))
	}

	resContentType := response.Header.Get("Content-Type")
	log.Printf("✅ Downloaded %d bytes (type: %s)", len(resBody), resContentType)
	return resBody, resContentType, nil
}

func (h *TelegramAPI) Explode(filePath interface{}) string {
	// تبدیل به string
	filePathStr, ok := filePath.(string)
	if !ok {
		log.Printf("⚠️ Explode: invalid filePath type: %T", filePath)
		return ""
	}

	log.Printf("🔍 Explode input: %s", filePathStr)

	// لیست کامل پوشه‌های ممکن در تلگرام (ترتیب مهمه!)
	knownDirs := []string{
		"photos",        // عکس‌ها
		"videos",        // ویدیوها
		"video_notes",   // ویدیو نوت‌ها
		"animations",    // GIF ها و انیمیشن‌ها
		"documents",     // فایل‌ها (شامل ویدیوهای بزرگ)
		"voice",         // ویس
		"audio",         // موزیک و صدا
		"music",         // موزیک (نسخه قدیمی)
		"stickers",      // استیکر
		"thumbnails",    // تصاویر کوچک
		"profile_photos", // عکس پروفایل
	}

	// روش 1: جستجوی پوشه‌های شناخته شده
	for _, dir := range knownDirs {
		// چک کن که این پوشه در مسیر وجود داره
		if strings.Contains(filePathStr, "/"+dir+"/") {
			// پیدا کردن آخرین موقعیت این پوشه (ممکنه چندبار تکرار شده باشه)
			idx := strings.LastIndex(filePathStr, "/"+dir+"/")
			if idx != -1 {
				// از شروع پوشه تا انتها رو برگردون (بدون / اول)
				result := filePathStr[idx+1:]
				log.Printf("✅ Found '%s' directory, extracted: %s", dir, result)
				return result
			}
		}
	}

	// روش 2: اگه مسیر کامل سرور داره، حذفش کن
	serverPaths := []string{
		"/var/www/html/bot/",
		"/var/www/html/",
		"/home/",
		"/opt/",
		"/bot/",
	}

	cleanPath := filePathStr
	for _, serverPath := range serverPaths {
		if strings.Contains(cleanPath, serverPath) {
			// پیدا کردن و حذف مسیر سرور
			idx := strings.Index(cleanPath, serverPath)
			if idx != -1 {
				cleanPath = cleanPath[idx+len(serverPath):]
				log.Printf("🔧 Removed server path: %s", serverPath)
				break
			}
		}
	}

	// حذف توکن از مسیر اگه وجود داره
	if strings.Contains(cleanPath, h.token) {
		parts := strings.Split(cleanPath, h.token)
		if len(parts) > 1 && parts[1] != "" {
			cleanPath = strings.TrimPrefix(parts[1], "/")
			log.Printf("🔧 Removed token, path now: %s", cleanPath)

			// دوباره چک کن برای پوشه‌های شناخته شده
			for _, dir := range knownDirs {
				if strings.HasPrefix(cleanPath, dir+"/") {
					log.Printf("✅ Found directory after token removal: %s", cleanPath)
					return cleanPath
				}
			}
		}
	}

	// روش 3: دو بخش آخر مسیر (folder/filename)
	parts := strings.Split(filePathStr, "/")
	var nonEmptyParts []string
	for _, part := range parts {
		if part != "" {
			nonEmptyParts = append(nonEmptyParts, part)
		}
	}

	if len(nonEmptyParts) >= 2 {
		// دو بخش آخر رو برگردون
		result := nonEmptyParts[len(nonEmptyParts)-2] + "/" + nonEmptyParts[len(nonEmptyParts)-1]

		// چک کن که آیا بخش اول یک پوشه شناخته شده هست
		folderName := nonEmptyParts[len(nonEmptyParts)-2]
		for _, dir := range knownDirs {
			if folderName == dir {
				log.Printf("✅ Using last two parts (recognized folder): %s", result)
				return result
			}
		}

		log.Printf("⚠️ Using last two parts (unrecognized folder): %s", result)
		return result
	}

	// اگه فقط یک بخش داریم
	if len(nonEmptyParts) == 1 {
		result := nonEmptyParts[0]
		log.Printf("⚠️ Only one part found: %s", result)
		return result
	}

	log.Printf("❌ Could not process path, returning as-is: %s", filePathStr)
	return filePathStr
}

func (h *TelegramAPI) UploadFile(contentType string, fileName string, data []byte, chatId string) (string, error) {
	// تعیین نوع فیلد بر اساس content type
	var formField string
	if strings.Contains(contentType, "image") {
		formField = "photo"
	} else if strings.Contains(contentType, "audio") {
		formField = "audio"
	} else if strings.Contains(contentType, "video") {
		formField = "video"
	} else {
		formField = "document"
	}

	// آماده‌سازی request body
	body := &bytes.Buffer{}
	mwriter := multipart.NewWriter(body)

	// تعیین URL endpoint
	var reqUrl string
	switch formField {
	case "photo":
		reqUrl = BaseUrl + "/bot" + h.token + "/sendPhoto"
	case "audio":
		reqUrl = BaseUrl + "/bot" + h.token + "/sendAudio"
	case "video":
		reqUrl = BaseUrl + "/bot" + h.token + "/sendVideo"
	default:
		reqUrl = BaseUrl + "/bot" + h.token + "/sendDocument"
	}

	// نوشتن chat_id
	if err := mwriter.WriteField("chat_id", chatId); err != nil {
		return "", fmt.Errorf("failed to write chat_id: %w", err)
	}

	// ایجاد فیلد فایل
	fileWriter, err := mwriter.CreateFormFile(formField, fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := fileWriter.Write(data); err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}

	// بستن multipart writer
	if err := mwriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// ایجاد HTTP request
	req, err := http.NewRequest("POST", reqUrl, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", mwriter.FormDataContentType())

	// ارسال request
	response, err := h.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}

	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if response.StatusCode != 200 {
		return "", fmt.Errorf("telegram upload failed (status %d): %s", response.StatusCode, string(resBody))
	}

	// پردازش JSON response
	var tgResponse map[string]interface{}
	if err := json.Unmarshal(resBody, &tgResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// چک کردن نتیجه
	ok, _ := tgResponse["ok"].(bool)
	if !ok {
		description, _ := tgResponse["description"].(string)
		return "", fmt.Errorf("telegram API error: %s", description)
	}

	// استخراج file_id بر اساس نوع فیلد
	result, ok := tgResponse["result"].(map[string]interface{})
	if !ok {
		return "", errors.New("invalid response format: missing result")
	}

	var fileID string
	if formField == "document" || formField == "video" || formField == "audio" {
		fileInfo, ok := result[formField].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("missing %s in response", formField)
		}
		fileID, _ = fileInfo["file_id"].(string)
	} else if formField == "photo" {
		photos, ok := result["photo"].([]interface{})
		if !ok || len(photos) == 0 {
			return "", errors.New("missing photo array in response")
		}
		// گرفتن بزرگترین عکس (آخرین در آرایه)
		lastPhoto, ok := photos[len(photos)-1].(map[string]interface{})
		if !ok {
			return "", errors.New("invalid photo format in response")
		}
		fileID, _ = lastPhoto["file_id"].(string)
	}

	if fileID == "" {
		return "", errors.New("file_id not found in response")
	}

	log.Printf("📤 Upload successful: %s (FileID: %s)", fileName, fileID)
	return fileID, nil
}

// متدهای با Context support
func (h *TelegramAPI) GetFileWithContext(ctx context.Context, fileId string) (string, error) {
	return h.GetFile(fileId)
}

func (h *TelegramAPI) DownloadFileWithContext(ctx context.Context, filePath string) ([]byte, string, error) {
	return h.DownloadFile(filePath)
}