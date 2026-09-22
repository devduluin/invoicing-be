package service

import (
	"encoding/json"
	"strings"

	domain "duluin_invoice/app/domain/documentconfig"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type DocumentConfigurationService struct{ repo domain.IRepository }

func NewDocumentConfigurationService(repo domain.IRepository) domain.IService {
	return &DocumentConfigurationService{repo: repo}
}

func (s *DocumentConfigurationService) List(companyID string) ([]domain.Item, error) {
	return s.repo.List(companyID)
}

func (s *DocumentConfigurationService) Get(companyID, docType string) (*domain.Item, error) {
	if !model.IsValidDocumentConfigType(docType) {
		return nil, &domain.ErrValidation{Message: "unknown document type"}
	}
	return s.repo.Get(companyID, docType)
}

func (s *DocumentConfigurationService) Reset(companyID, docType string) error {
	if !model.IsValidDocumentConfigType(docType) {
		return &domain.ErrValidation{Message: "unknown document type"}
	}
	return s.repo.Reset(companyID, docType)
}

const (
	maxLabelLen = 120
	maxKeyLen   = 40
	maxKeys     = 80
	maxImageLen = 600_000
)

func cleanKey(k string) bool {
	if k == "" || len(k) > maxKeyLen {
		return false
	}
	for _, r := range k {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

// Save validates and normalises the configuration, then stores it for (company, docType). Rich-text
// defaults are sanitised the same way document notes are.
func (s *DocumentConfigurationService) Save(companyID, actorID, docType string, cfg *domain.Config) (*domain.Item, error) {
	if !model.IsValidDocumentConfigType(docType) {
		return nil, &domain.ErrValidation{Message: "unknown document type"}
	}
	if cfg.Language != "" && cfg.Language != "id" && cfg.Language != "en" {
		return nil, &domain.ErrValidation{Message: "language must be id or en"}
	}
	cfg.DocumentName = strings.TrimSpace(cfg.DocumentName)
	if len(cfg.DocumentName) > maxLabelLen {
		return nil, &domain.ErrValidation{Message: "document name is too long"}
	}
	if len(cfg.Labels) > maxKeys || len(cfg.Hidden) > maxKeys || len(cfg.Shown) > maxKeys || len(cfg.ColumnOrder) > maxKeys {
		return nil, &domain.ErrValidation{Message: "too many entries"}
	}
	labels := make(map[string]string, len(cfg.Labels))
	for k, v := range cfg.Labels {
		v = strings.TrimSpace(v)
		if !cleanKey(k) || len(v) > maxLabelLen {
			return nil, &domain.ErrValidation{Message: "invalid label"}
		}
		if v != "" {
			labels[k] = v
		}
	}
	cfg.Labels = labels
	for _, k := range append(append(append([]string{}, cfg.Hidden...), cfg.Shown...), cfg.ColumnOrder...) {
		if !cleanKey(k) {
			return nil, &domain.ErrValidation{Message: "invalid field key"}
		}
	}
	for _, b := range []*domain.Block{&cfg.Notes, &cfg.Terms} {
		b.Label = strings.TrimSpace(b.Label)
		if len(b.Label) > maxLabelLen {
			return nil, &domain.ErrValidation{Message: "label is too long"}
		}
		b.Content = utils.SanitizeRichText(b.Content)
		if len(b.Content) > 20_000 {
			return nil, &domain.ErrValidation{Message: "default text is too long"}
		}
	}
	cfg.Signature.Name = strings.TrimSpace(cfg.Signature.Name)
	if len(cfg.Signature.Name) > maxLabelLen {
		return nil, &domain.ErrValidation{Message: "signature name is too long"}
	}
	if img := cfg.Signature.Image; img != "" && (!strings.HasPrefix(img, "data:image/") || len(img) > maxImageLen) {
		return nil, &domain.ErrValidation{Message: "signature image must be an image under 600 KB"}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return s.repo.Save(companyID, docType, string(raw), actorID)
}
