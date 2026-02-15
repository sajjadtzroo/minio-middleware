package models

type GenericResponse struct {
	Result  bool   `json:"result"`
	Message string `json:"message"`
}

type UploadedResponse struct {
	Result bool   `json:"result"`
	FileId string `json:"fileId"`
}
