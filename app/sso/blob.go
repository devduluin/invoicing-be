package sso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSO routes files through its Minio storage. The X-Forwarded-Host header
// selects the bucket (see MinioServiceController::getBucketFromHost).
const (
	blobUploadPath = "/users/file_blob_uploader"
	blobDeletePath = "/users/file_delete"
	storageHost    = "invoice"
)

// UploadBlob pushes a `data:<mime>;base64,<...>` string to SSO storage and
// returns the stored public URL. folder is namespaced by the caller
// (e.g. "<companyID>/company_logos"). Mirrors acc-master's UploadBlobToSSO.
func (c *Client) UploadBlob(ctx context.Context, token, dataURI, folder string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"file": dataURI, "folder": folder})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+blobUploadPath, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build blob upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", storageHost)
	if auth := bearer(token, c.apiKey); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", &Error{Path: blobUploadPath, Message: err.Error(), Temporary: true}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &Error{Path: blobUploadPath, Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
	}

	var body struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		File    string `json:"file"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", &Error{Path: blobUploadPath, Message: "decode: " + err.Error()}
	}
	if !body.Success || strings.TrimSpace(body.File) == "" {
		msg := body.Message
		if msg == "" {
			msg = "sso did not return a file url"
		}
		return "", &Error{Path: blobUploadPath, Message: msg}
	}
	return body.File, nil
}

// DeleteBlob removes a previously uploaded file by its stored URL. Best-effort:
// a missing file is not treated as an error by the caller.
func (c *Client) DeleteBlob(ctx context.Context, token, fileURL string) error {
	fileURL = strings.TrimSpace(fileURL)
	if fileURL == "" {
		return nil
	}
	payload, _ := json.Marshal(map[string]string{"filename": fileURL})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+blobDeletePath, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build blob delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Forwarded-Host", storageHost)
	if auth := bearer(token, c.apiKey); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return &Error{Path: blobDeletePath, Message: err.Error(), Temporary: true}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Path: blobDeletePath, Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
	}
	return nil
}
