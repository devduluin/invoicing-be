package utils

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const MaxAttachmentFileSize = 5 * 1024 * 1024

var allowedAttachmentExts = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".pdf":  {},
}

// IsAttachmentURL reports whether value is already a hosted http(s) URL rather than a base64
// data: URI waiting to be uploaded.
func IsAttachmentURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

// ValidateAttachmentURL checks an already-hosted attachment URL's extension against the allow-list.
func ValidateAttachmentURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid attachment URL")
	}
	if !isAllowedAttachmentExt(strings.ToLower(path.Ext(parsed.Path))) {
		return fmt.Errorf("attachment URL must use one of: .jpg, .jpeg, .png, .pdf")
	}
	return nil
}

// ValidateBase64Attachment decodes and checks a data: URI (or bare base64) attachment: size limit
// and an allow-listed image/PDF type, by declared media type or by sniffing its file signature.
func ValidateBase64Attachment(value string) error {
	mediaType, encoded := splitDataURL(value)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(encoded)
	}
	if err != nil {
		return fmt.Errorf("attachment must be a URL or valid base64 file")
	}
	if len(decoded) == 0 {
		return fmt.Errorf("attachment is empty")
	}
	if len(decoded) > MaxAttachmentFileSize {
		return fmt.Errorf("attachment exceeds 5MB")
	}
	if !isAllowedAttachmentContent(mediaType, decoded) {
		return fmt.Errorf("attachment must be .jpg, .jpeg, .png, or .pdf")
	}
	return nil
}

func splitDataURL(value string) (string, string) {
	if !strings.HasPrefix(value, "data:") {
		return "", strings.TrimSpace(value)
	}
	parts := strings.SplitN(value, ",", 2)
	if len(parts) != 2 {
		return "", value
	}
	metadata := strings.TrimPrefix(parts[0], "data:")
	mediaType := strings.Split(metadata, ";")[0]
	return strings.ToLower(mediaType), strings.TrimSpace(parts[1])
}

func isAllowedAttachmentContent(mediaType string, data []byte) bool {
	switch mediaType {
	case "image/jpeg", "image/jpg", "image/png", "application/pdf":
		return true
	case "":
		return hasJPEGSignature(data) || hasPNGSignature(data) || hasPDFSignature(data)
	default:
		return false
	}
}

func isAllowedAttachmentExt(ext string) bool {
	_, ok := allowedAttachmentExts[ext]
	return ok
}

func hasJPEGSignature(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
}

func hasPNGSignature(data []byte) bool {
	return len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 &&
		data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a
}

func hasPDFSignature(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == "%PDF"
}
