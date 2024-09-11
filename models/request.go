package models

type DownLoadFromLinkRequest struct {
	Link     string `json:"link"`
	Bucket   string `json:"bucket"`
	FileName string `json:"fileName"`
}
