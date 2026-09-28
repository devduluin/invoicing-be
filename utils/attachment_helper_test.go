package utils

import (
	"encoding/base64"
	"testing"
)

func TestIsAttachmentURL(t *testing.T) {
	if !IsAttachmentURL("https://minio.example/files/a.png") {
		t.Error("https URL should be recognized as an attachment URL")
	}
	if IsAttachmentURL("data:image/png;base64,abc") {
		t.Error("a data: URI must not be treated as an attachment URL")
	}
}

func TestValidateAttachmentURL_RejectsDisallowedExtension(t *testing.T) {
	if err := ValidateAttachmentURL("https://minio.example/files/a.exe"); err == nil {
		t.Fatal("want an error for a non-image/pdf URL")
	}
	if err := ValidateAttachmentURL("https://minio.example/files/a.pdf"); err != nil {
		t.Fatalf("a .pdf URL should be allowed: %v", err)
	}
}

func TestValidateBase64Attachment_RejectsOversizedFile(t *testing.T) {
	big := make([]byte, MaxAttachmentFileSize+1)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(big)
	if err := ValidateBase64Attachment(dataURL); err == nil {
		t.Fatal("want an error for a file over 5MB")
	}
}

func TestValidateBase64Attachment_RejectsDisallowedType(t *testing.T) {
	dataURL := "data:application/zip;base64," + base64.StdEncoding.EncodeToString([]byte("PK\x03\x04"))
	if err := ValidateBase64Attachment(dataURL); err == nil {
		t.Fatal("want an error for a non-image/pdf content type")
	}
}

func TestValidateBase64Attachment_AcceptsAValidPNG(t *testing.T) {
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	if err := ValidateBase64Attachment("data:image/png;base64," + png); err != nil {
		t.Fatalf("valid PNG should be accepted: %v", err)
	}
}
