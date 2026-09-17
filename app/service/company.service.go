package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"strings"

	domain "duluin_invoice/app/domain/company"
	"duluin_invoice/app/model"
)

const (
	maxLogoBytes = 2 * 1024 * 1024 // 2MB, matches acc-frontend
	logoFolder   = "company_logos"
)

// companyStore is the slice of company persistence this service needs.
type companyStore interface {
	FindCompanyByID(id string) (*model.Company, error)
	SaveCompany(c *model.Company) error
}

// blobStore is the SSO storage slice (satisfied by *sso.Client).
type blobStore interface {
	UploadBlob(ctx context.Context, token, dataURI, folder string) (string, error)
	DeleteBlob(ctx context.Context, token, fileURL string) error
}

type CompanyService struct {
	repo companyStore
	blob blobStore
}

func NewCompanyService(repo companyStore, blob blobStore) *CompanyService {
	return &CompanyService{repo: repo, blob: blob}
}

var _ domain.ICompanyService = (*CompanyService)(nil)

// UpdateProfile applies the company-settings form to the active company. The
// logo goes through SSO storage exactly like acc-master: a `data:` URI is
// uploaded, an unchanged URL is kept, "" removes it (old file deleted).
func (s *CompanyService) UpdateProfile(a domain.Actor, dto domain.UpdateProfileDTO) (*model.Company, error) {
	companyID := strings.TrimSpace(a.ActiveCompanyID)
	if companyID == "" {
		return nil, &domain.ErrCompanyNotFound{}
	}
	company, err := s.repo.FindCompanyByID(companyID)
	if err != nil {
		return nil, err
	}
	if company == nil {
		return nil, &domain.ErrCompanyNotFound{}
	}

	if dto.CompanyLogo != nil {
		next, err := s.processLogo(a.Token, company.CompanyLogo, strings.TrimSpace(*dto.CompanyLogo), company.ID+"/"+logoFolder)
		if err != nil {
			return nil, err
		}
		company.CompanyLogo = next
	}

	applyString(&company.Name, dto.Name, true)
	applyString(&company.Email, dto.Email, false)
	applyString(&company.Phone, dto.Phone, false)
	applyString(&company.Npwp, dto.Npwp, false)
	applyString(&company.Alamat, dto.Alamat, false)
	applyString(&company.Kota, dto.Kota, false)
	applyString(&company.Provinsi, dto.Provinsi, false)
	applyString(&company.KodePos, dto.KodePos, false)
	company.UpdatedBy = a.UserID

	if err := s.repo.SaveCompany(company); err != nil {
		return nil, err
	}
	return company, nil
}

// processLogo returns the URL to persist. old is the current stored URL.
func (s *CompanyService) processLogo(token, old, next, folder string) (string, error) {
	switch {
	case next == "":
		s.deleteLogo(token, old)
		return "", nil
	case next == old || isHTTPURL(next):
		return next, nil // unchanged, or an already-hosted URL — keep as-is
	case strings.HasPrefix(next, "data:"):
		if err := validateLogoDataURI(next); err != nil {
			return "", err
		}
		uploaded, err := s.blob.UploadBlob(context.Background(), token, next, folder)
		if err != nil {
			return "", fmt.Errorf("upload logo: %w", err)
		}
		s.deleteLogo(token, old) // best-effort cleanup of the replaced file
		return uploaded, nil
	default:
		return "", &domain.ErrValidation{Message: "invalid logo format"}
	}
}

func (s *CompanyService) deleteLogo(token, old string) {
	if old == "" {
		return
	}
	if err := s.blob.DeleteBlob(context.Background(), token, old); err != nil {
		log.Printf("[company] delete old logo failed url=%s err=%v", old, err)
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func applyString(dst *string, src *string, trimOnly bool) {
	if src == nil {
		return
	}
	v := strings.TrimSpace(*src)
	if trimOnly && v == "" {
		return // never blank the name
	}
	*dst = v
}

func isHTTPURL(v string) bool {
	u, err := url.ParseRequestURI(v)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func validateLogoDataURI(v string) error {
	comma := strings.IndexByte(v, ',')
	if comma < 0 {
		return &domain.ErrValidation{Message: "invalid logo data"}
	}
	meta, b64 := v[5:comma], v[comma+1:]
	mime := strings.SplitN(meta, ";", 2)[0]
	switch mime {
	case "image/png", "image/jpeg", "image/jpg":
	default:
		return &domain.ErrValidation{Message: "logo must be PNG or JPG"}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(b64))
	}
	if err != nil || len(decoded) == 0 {
		return &domain.ErrValidation{Message: "logo data can't be read"}
	}
	if len(decoded) > maxLogoBytes {
		return &domain.ErrValidation{Message: "logo size must not exceed 2MB"}
	}
	return nil
}
