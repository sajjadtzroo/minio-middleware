package telegram_api

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const BaseUrl = "http://94.130.99.214"
const ContentType = "application/json"

type TelegramAPI struct {
	*http.Client
	token string
}

func New(token string) *TelegramAPI {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
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

func (http *TelegramAPI) GetFile(fileId string) (string, error) {
	bodyRaw := map[string]string{
		"file_id": fileId,
	}
	reqURL := BaseUrl + "/bot" + http.token + "/getFile"
	body, err := json.Marshal(bodyRaw)
	if err != nil {
		return "", err
	}

	response, err := http.Client.Post(reqURL, ContentType, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	defer response.Body.Close()
	resBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		log.Printf("token is: " + http.token)
		return "", errors.New("telegram failed " + string(resBody))
	}

	var result GetFileResponse
	errJson := json.Unmarshal(resBody, &result)
	if errJson != nil {
		return "", errJson
	}

	return result.Result.FilePath, nil
}

func (http *TelegramAPI) DownloadFile(filePath string) (io.Reader, error) {
	reqURL := BaseUrl + "/file/" + http.token + filePath
	response, err := http.Client.Get(reqURL)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != 200 {
		return nil, errors.New("telegram failed")
	}

	return response.Body, nil
}

func (http *TelegramAPI) Explode(filePath string) string {
	data := strings.Split(filePath, http.token)

	if strings.Contains(data[1], ".") {
		return strings.Split(data[1], ".")[0]
	}

	return data[1]
}
