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
	"os"
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
	cleanPath := strings.TrimPrefix(filePath, "/")

	// چک کن که آیا دانلود مستقیم از تلگرام فعال است
	directDownload := os.Getenv("TELEGRAM_DIRECT_DOWNLOAD") == "true"

	var reqURL string

	if directDownload {
		// دانلود مستقیم از API تلگرام
		reqURL = "https://api.telegram.org/file/bot" + h.token + "/" + cleanPath
		log.Printf("📥 Direct download from Telegram API: %s", reqURL)
	} else {
		// دانلود از پروکسی/سرور محلی
		reqURL = BaseUrl + "/file/" + h.token + "/" + cleanPath
		log.Printf("📥 Downloading from proxy server: %s", reqURL)
	}

	// اولین تلاش
	response, err := h.client.Get(reqURL)
	if err != nil {
		return nil, "", fmt.Errorf("request failed: %w", err)
	}

	defer response.Body.Close()

	// اگه موفق بود
	if response.StatusCode == 200 {
		resBody, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read response: %w", err)
		}

		resContentType := response.Header.Get("Content-Type")
		source := "proxy"
		if directDownload {
			source = "Telegram API"
		}
		log.Printf("✅ Downloaded %d bytes from %s (type: %s)", len(resBody), source, resContentType)
		return resBody, resContentType, nil
	}

	// اگه 404 بود و از پروکسی بود
	if response.StatusCode == 404 && !directDownload {
		log.Printf("⚠️ Proxy returned 404, checking retry strategy...")

		// اگه env variable برای retry تنظیم شده
		retryOnFail := os.Getenv("TELEGRAM_RETRY_ON_PROXY_FAIL") == "true"

		if retryOnFail {
			// صبر کن شاید فایل در حال آماده شدن باشه
			retryCount := 3
			retryDelayStr := os.Getenv("TELEGRAM_RETRY_DELAY")
			retryDelay := 2 * time.Second
			if retryDelayStr != "" {
				if seconds, err := time.ParseDuration(retryDelayStr + "s"); err == nil {
					retryDelay = seconds
				}
			}

			for i := 1; i <= retryCount; i++ {
				log.Printf("⏳ Retry %d/%d after %v...", i, retryCount, retryDelay)
				time.Sleep(retryDelay)

				retryResp, err := h.client.Get(reqURL)
				if err == nil && retryResp.StatusCode == 200 {
					defer retryResp.Body.Close()
					resBody, _ := io.ReadAll(retryResp.Body)
					log.Printf("✅ Downloaded from proxy on retry %d", i)
					return resBody, retryResp.Header.Get("Content-Type"), nil
				}
				if retryResp != nil {
					retryResp.Body.Close()
				}
			}
		}

		// اگه retry هم کار نکرد، fallback به API تلگرام
		fallbackEnabled := os.Getenv("TELEGRAM_FALLBACK_TO_API") != "false" // default true

		if fallbackEnabled {
			log.Printf("🔄 Falling back to Telegram API...")
			fallbackURL := "https://api.telegram.org/file/bot" + h.token + "/" + cleanPath

			fallbackResp, err := h.client.Get(fallbackURL)
			if err != nil {
				return nil, "", fmt.Errorf("fallback to Telegram API also failed: %w", err)
			}
			defer fallbackResp.Body.Close()

			if fallbackResp.StatusCode == 200 {
				resBody, _ := io.ReadAll(fallbackResp.Body)
				log.Printf("✅ Downloaded from Telegram API (fallback)")
				return resBody, fallbackResp.Header.Get("Content-Type"), nil
			}

			return nil, "", fmt.Errorf("both proxy and Telegram API failed (status %d)", fallbackResp.StatusCode)
		}
	}

	return nil, "", fmt.Errorf("download failed (status %d) from %s", response.StatusCode, reqURL)
}

func (h *TelegramAPI) Explode(filePath interface{}) string {
	// تبدیل به string
	filePathStr, ok := filePath.(string)
	if !ok {
		log.Printf("⚠️ Explode: invalid filePath type: %T", filePath)
		return ""
	}

	log.Printf("🔍 Explode input: %s", filePathStr)

	// لیست کامل پوشه‌های ممکن در تلگرام
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
			// پیدا کردن آخرین موقعیت این پوشه
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
		log.Printf("✅ Using last two parts: %s", result)
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

	// استخراج file_id
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