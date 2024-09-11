package models

type DownLoadFromLinkRequest struct {
	Link     string `json:"link,omitempty"`
	Bucket   string `json:"bucket,omitempty"`
	FileName string `json:"fileName,omitempty"`
}
