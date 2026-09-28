package sso

import (
	"context"
	"log"
	"strings"

	"duluin_invoice/utils"
)

// ProcessAttachment validates then uploads a single document attachment/signature field (Sales/
// Purchase Order/Invoice, Delivery Note, Goods Receipt — every AttachmentData/SignatureData column)
// to SSO's blob storage (MinIO), same UploadBlob this package already uses for the company logo. An
// already-hosted URL is returned unchanged (never re-uploaded); a blank value returns "".
func (c *Client) ProcessAttachment(ctx context.Context, token, value, folder string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if utils.IsAttachmentURL(value) {
		if err := utils.ValidateAttachmentURL(value); err != nil {
			return "", err
		}
		return value, nil
	}
	if err := utils.ValidateBase64Attachment(value); err != nil {
		return "", err
	}
	return c.UploadBlob(ctx, token, value, folder)
}

// ProcessUpdatedAttachment uploads a changed attachment and deletes the previous one it replaced.
// The old file is deleted only once the new one uploaded successfully, and never when the value is
// unchanged — editing a document without touching its attachment must not touch storage at all.
func (c *Client) ProcessUpdatedAttachment(ctx context.Context, token, existing, next, folder string) (string, error) {
	uploaded, err := c.ProcessAttachment(ctx, token, next, folder)
	if err != nil {
		return "", err
	}
	existing = strings.TrimSpace(existing)
	if existing != "" && existing != uploaded && utils.IsAttachmentURL(existing) {
		if err := c.DeleteBlob(ctx, token, existing); err != nil {
			// Best-effort: an orphaned blob costs storage, not correctness — never fail the save over it.
			log.Printf("[sso] attachment cleanup: failed to delete %q: %v", existing, err)
		}
	}
	return uploaded, nil
}
