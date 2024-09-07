package models

type HealthCheckResponse struct {
	Result  bool   `json:"result"`
	Message string `json:"message"`
	Ip      string `json:"ip"`
}

type GenericResponse struct {
	Result  bool   `json:"result"`
	Message string `json:"message"`
}

type UploadedResponse struct {
	Result bool   `json:"result"`
	FileId string `json:"fileId"`
}
