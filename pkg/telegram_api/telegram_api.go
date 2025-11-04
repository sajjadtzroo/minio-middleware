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
	"net"
	"net/http"
	"strings"
	"time"
)

// Dynamic URL selection based on proxy availability
func getBaseURL() string {
	proxyURL := "http://94.130.99.214"
	officialURL := "https://api.telegram.org"

	// Check if proxy is available with short timeout
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   1 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	testURL := fmt.Sprintf("%s/bot123/test", proxyURL)
	resp, err := client.Get(testURL)
	if err == nil {
		resp.Body.Close()
		log.Printf("✅ Using proxy URL: %s", proxyURL)
		return proxyURL
	}

	log.Printf("⚠️ Proxy unavailable (%v), using official API: %s", err, officialURL)
	return officialURL
}

var BaseUrl = getBaseURL()
const ContentType = "application/json"

type TelegramAPI struct {
	client *http.Client
	token  string
}

func New(token string) *TelegramAPI {
	// Optimized HTTP client with connection pooling
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        100,
			MaxConnsPerHost:     100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: 60 * time.Second, // Overall timeout
	}

	api := TelegramAPI{
		client: client,
		token:  token,
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
	} `json:"result"`
	Description string `json:"description,omitempty"`
}

func (h *TelegramAPI) GetFile(fileId string) (string, error) {
	return h.GetFileWithContext(context.Background(), fileId)
}

// GetFileWithContext - Version with context support for timeouts
func (h *TelegramAPI) GetFileWithContext(ctx context.Context, fileId string) (string, error) {
	bodyRaw := map[string]string{
		"file_id": fileId,
	}
	reqURL := BaseUrl + "/bot" + h.token + "/getFile"
	body, err := json.Marshal(bodyRaw)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", ContentType)

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
		return "", fmt.Errorf("telegram API error (status %d): %s", response.StatusCode, string(resBody))
	}

	var result GetFileResponse
	if err := json.Unmarshal(resBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Ok {
		return "", fmt.Errorf("telegram API returned error: %s", result.Description)
	}

	if result.Result.FilePath == "" {
		return "", errors.New("empty file path in response")
	}

	log.Printf("📁 GetFile successful: %s (size: %d bytes)", result.Result.FilePath, result.Result.FileSize)
	return result.Result.FilePath, nil
}

func (h *TelegramAPI) DownloadFile(filePath string) ([]byte, string, error) {
	return h.DownloadFileWithContext(context.Background(), filePath)
}

// DownloadFileWithContext - Version with context support for timeouts
func (h *TelegramAPI) DownloadFileWithContext(ctx context.Context, filePath string) ([]byte, string, error) {
	reqURL := BaseUrl + "/file/bot" + h.token + filePath

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Use a separate client with longer timeout for downloads
	downloadClient := &http.Client{
		Transport: h.client.Transport,
		Timeout:   120 * time.Second, // Longer timeout for large files
	}

	response, err := downloadClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download request failed: %w", err)
	}
	defer response.Body.Close()

	// Read response with size limit
	const maxSize = 50 * 1024 * 1024 // 50MB max
	limitedReader := io.LimitReader(response.Body, maxSize)

	resBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file data: %w", err)
	}

	if response.StatusCode != 200 {
		return nil, "", fmt.Errorf("download failed (status %d): %s", response.StatusCode, string(resBody))
	}

	resContentType := response.Header.Get("Content-Type")
	log.Printf("📥 Downloaded %d bytes (type: %s)", len(resBody), resContentType)

	return resBody, resContentType, nil
}

func (h *TelegramAPI) Explode(filePath string) string {
	// Remove the token from the file path if present
	parts := strings.Split(filePath, h.token)
	if len(parts) > 1 {
		return parts[1]
	}
	// If token not found, check for "/file/bot" prefix
	if strings.HasPrefix(filePath, "/file/bot") {
		return strings.TrimPrefix(filePath, "/file/bot"+h.token)
	}
	return filePath
}

func (h *TelegramAPI) UploadFile(contentType string, fileName string, data []byte, chatId string) (string, error) {
	return h.UploadFileWithContext(context.Background(), contentType, fileName, data, chatId)
}

// UploadFileWithContext - Version with context support for timeouts
func (h *TelegramAPI) UploadFileWithContext(ctx context.Context, contentType string, fileName string, data []byte, chatId string) (string, error) {
	// Determine the appropriate form field based on content type
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

	// Prepare the request body
	body := &bytes.Buffer{}
	mwriter := multipart.NewWriter(body)

	// Define the request URL
	reqUrl := BaseUrl + "/bot" + h.token + "/send" + strings.Title(formField)

	// Write the 'chat_id' field
	if err := mwriter.WriteField("chat_id", chatId); err != nil {
		return "", fmt.Errorf("failed to write chat_id: %w", err)
	}

	// Create file field
	fileWriter, err := mwriter.CreateFormFile(formField, fileName)
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := fileWriter.Write(data); err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}

	// Close the multipart writer
	if err := mwriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", reqUrl, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", mwriter.FormDataContentType())

	// Use a separate client with longer timeout for uploads
	uploadClient := &http.Client{
		Transport: h.client.Transport,
		Timeout:   180 * time.Second, // 3 minutes for large uploads
	}

	// Send the request
	response, err := uploadClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer response.Body.Close()

	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if response.StatusCode != 200 {
		return "", fmt.Errorf("telegram upload failed (status %d): %s", response.StatusCode, string(resBody))
	}

	// Parse the JSON response
	var tgResponse map[string]interface{}
	if err := json.Unmarshal(resBody, &tgResponse); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check if the response is OK
	ok, _ := tgResponse["ok"].(bool)
	if !ok {
		description, _ := tgResponse["description"].(string)
		return "", fmt.Errorf("telegram API error: %s", description)
	}

	// Extract file_id based on the field type
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
		// Get the largest photo (last in array)
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