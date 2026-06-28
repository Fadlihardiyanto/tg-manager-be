package model

type UploadPresignRequest struct {
	FileName    string `json:"file_name" validate:"required,min=3"`
	ContentType string `json:"content_type" validate:"required,oneof=image/jpeg image/png image/webp application/pdf"`
}

type UploadPresignResponse struct {
	UploadURL string `json:"upload_url"`
	PublicURL string `json:"public_url"`
	Key       string `json:"key"`
}
