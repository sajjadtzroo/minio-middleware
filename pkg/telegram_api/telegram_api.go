package telegram_api

import (
	"bytes"
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

type GetFileResponse struct {
	Result struct {
		FilePath string `json:"file_path"`
	} `json:"result"`
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

	return result.Result.FilePath, nil
}

func (h *TelegramAPI) DownloadFile(filePath string) ([]byte, string, error) {
	reqURL := BaseUrl + "/file/" + h.token + filePath
	log.Printf("Request URL: %s", reqURL)

	response, err := h.client.Get(reqURL)
	if err != nil {
		return nil, "", err
	}

	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}

	if response.StatusCode != 200 {
		return nil, "", fmt.Errorf("telegram failed: %s", string(resBody))
	}

	resContentType := response.Header.Get("Content-Type")
	return resBody, resContentType, nil
}

func (h *TelegramAPI) Explode(filePath string) string {
	data := strings.Split(filePath, h.token)

	//if strings.Contains(data[1], ".") {
	//	return strings.Split(data[1], ".")[0]
	//}

	return data[1]
}

func (h *TelegramAPI) UploadFile(contentType string, fileName string, data []byte, chatId string) (string, error) {
	// Write the file part
	var formField string
	if strings.Contains(contentType, "image") {
		formField = "photo"
	} else if strings.Contains(contentType, "audio") {
		formField = "audio"
	} else if strings.Contains(contentType, "video") {
		formField = "video"
	} else if strings.Contains(contentType, "text") {
		formField = "document"
	} else {
		formField = "document"
	}

	// Prepare the request body
	body := &bytes.Buffer{}
	mwriter := multipart.NewWriter(body)

	// Define the request URL
	reqUrl := BaseUrl + "/bot" + h.token + "/send" + formField

	// Write the 'chat_id' field
	err := mwriter.WriteField("chat_id", chatId)
	if err != nil {
		return "", err
	}

	fileWriter, err := mwriter.CreateFormFile(formField, fileName)
	if err != nil {
		return "", err
	}

	_, err = fileWriter.Write(data)
	if err != nil {
		return "", err
	}

	// Close the multipart writer to finalize the form data
	mwriter.Close()

	// Create the HTTP request
	req, err := http.NewRequest("POST", reqUrl, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", mwriter.FormDataContentType())

	// Send the request
	response, err := h.client.Do(req)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()
	resBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if response.StatusCode != 200 {
		return "", fmt.Errorf("telegram failed with status %s and body is: %s", response.Status, string(resBody))
	}

	// Parse the JSON response
	var tgResponse map[string]interface{}
	err = json.Unmarshal(resBody, &tgResponse)
	if err != nil {
		return "", err
	}

	if !tgResponse["ok"].(bool) {
		return "", fmt.Errorf("telegram API returned an error: %s", string(resBody))
	}

	// Get the file_id from the result
	if formField == "document" || formField == "video" {
		result := tgResponse["result"].(map[string]interface{})
		resultList := result[formField].(map[string]interface{})

		// Get the file_id
		fileID := resultList["file_id"].(string)

		return fileID, nil
	} else {
		result := tgResponse["result"].(map[string]interface{})
		resultList := result[formField].([]interface{})

		// Retrieve the last (largest) photo
		lastPhoto := resultList[len(resultList)-1].(map[string]interface{})

		// Get the file_id
		fileID := lastPhoto["file_id"].(string)

		return fileID, nil

	}

}
