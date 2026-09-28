package controller

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"duluin_invoice/app/sso"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

// uploadDocumentAttachment turns a document's AttachmentData/SignatureData field from a base64
// data: URI into a MinIO URL (via SSO's blob uploader), in place, using the request's own
// Authorization header. `existing` is the document's CURRENT stored value ("" on create); when the
// field is unchanged, its old URL is kept and nothing is re-uploaded — the repository always
// persists a URL, never base64. Call it once per attachment-bearing field, right after BodyParser +
// validation and before the service call. Same convention as CompanyService.processLogo
// (context.Background(): the upload/delete round trip to SSO outlives the fiber request context).
//
// docFolder is namespaced under the caller's company (e.g. "<companyID>/sales-invoice"), same
// grouping company.service.go's logo upload and acc-master-service's asset attachments already use
// — every company's files live under their own prefix in the bucket, so nothing collides or needs
// cross-company filtering to clean up.
func uploadDocumentAttachment(c *fiber.Ctx, ssoClient *sso.Client, docFolder, existing string, value *string) error {
	folder := middlewares.GetCompanyID(c) + "/" + docFolder
	uploaded, err := ssoClient.ProcessUpdatedAttachment(context.Background(), c.Get("Authorization"), existing, *value, folder)
	if err != nil {
		return err
	}
	*value = uploaded
	return nil
}

// attachmentUploadFailed answers a request whose attachment/signature couldn't be validated or
// uploaded — a client error (bad file type, too large, SSO unreachable), never a 500.
func attachmentUploadFailed(c *fiber.Ctx, err error) error {
	return utils.ValidationFailed(c, []string{err.Error()})
}
