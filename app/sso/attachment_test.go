package sso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestProcessAttachment_KeepsAnExistingURLAsIs(t *testing.T) {
	c := NewClient("http://unused.invalid", "duluin_invoice", "")
	got, err := c.ProcessAttachment(context.Background(), "", "https://minio.example/files/a.png", "sales-invoice")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://minio.example/files/a.png" {
		t.Fatalf("got %q", got)
	}
}

func TestProcessAttachment_BlankIsBlank(t *testing.T) {
	c := NewClient("http://unused.invalid", "duluin_invoice", "")
	got, err := c.ProcessAttachment(context.Background(), "", "", "sales-invoice")
	if err != nil || got != "" {
		t.Fatalf("got %q, err %v", got, err)
	}
}

func newBlobServer(t *testing.T, uploadedURL string) (*Client, *[]string) {
	t.Helper()
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case blobUploadPath:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "file": uploadedURL})
		case blobDeletePath:
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			deleted = append(deleted, body["filename"])
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "duluin_invoice", ""), &deleted
}

func TestProcessAttachment_UploadsANewBase64File(t *testing.T) {
	c, _ := newBlobServer(t, "https://minio.example/files/new.png")
	got, err := c.ProcessAttachment(context.Background(), "", "data:image/png;base64,"+pngB64, "sales-invoice")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://minio.example/files/new.png" {
		t.Fatalf("got %q", got)
	}
}

func TestProcessUpdatedAttachment_DeletesThePreviousFileOnlyWhenItChanged(t *testing.T) {
	c, deleted := newBlobServer(t, "https://minio.example/files/new.png")

	got, err := c.ProcessUpdatedAttachment(context.Background(), "", "https://minio.example/files/old.png", "data:image/png;base64,"+pngB64, "sales-invoice")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://minio.example/files/new.png" {
		t.Fatalf("got %q", got)
	}
	if len(*deleted) != 1 || (*deleted)[0] != "https://minio.example/files/old.png" {
		t.Fatalf("want the old file deleted once, got %v", *deleted)
	}

	*deleted = nil
	got, err = c.ProcessUpdatedAttachment(context.Background(), "", "https://minio.example/files/old.png", "https://minio.example/files/old.png", "sales-invoice")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://minio.example/files/old.png" || len(*deleted) != 0 {
		t.Fatalf("unchanged attachment must not upload or delete: got %q deleted=%v", got, *deleted)
	}
}
