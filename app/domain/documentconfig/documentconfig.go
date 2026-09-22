// Package domain_documentconfig — per-company, per-document-type configuration of what a document
// prints (name, labels, visible fields and columns, notes/terms defaults, signature, language).
package domain_documentconfig

import "encoding/json"

// Block — a notes / terms section: shown or not, its heading, and the default content that seeds
// NEW documents (existing documents keep their own notes / terms).
type Block struct {
	Show    *bool  `json:"show,omitempty"`
	Label   string `json:"label,omitempty"`
	Content string `json:"content,omitempty"`
}

// Signature — whether the signature block prints, the name under it, and a default image that
// seeds new documents (documents keep the signature they were saved with).
type Signature struct {
	Show  *bool  `json:"show,omitempty"`
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`
}

// Config is the shared schema for every document type.
type Config struct {
	Language     string            `json:"language,omitempty"` // "id" | "en"
	DocumentName string            `json:"documentName,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Hidden       []string          `json:"hidden,omitempty"`
	Shown        []string          `json:"shown,omitempty"`
	ColumnOrder  []string          `json:"columnOrder,omitempty"`
	Notes        Block             `json:"notes"`
	Terms        Block             `json:"terms"`
	Signature    Signature         `json:"signature"`
}

// Item is what the API returns for one document type.
type Item struct {
	DocType   string          `json:"doc_type"`
	Config    json.RawMessage `json:"config"`
	IsDefault bool            `json:"is_default"`
	UpdatedAt *string         `json:"updated_at,omitempty"`
}

type IRepository interface {
	List(companyID string) ([]Item, error)
	Get(companyID, docType string) (*Item, error)
	Save(companyID, docType, configJSON, actorID string) (*Item, error)
	Reset(companyID, docType string) error
}

type IService interface {
	List(companyID string) ([]Item, error)
	Get(companyID, docType string) (*Item, error)
	Save(companyID, actorID, docType string, cfg *Config) (*Item, error)
	Reset(companyID, docType string) error
}

type ErrValidation struct{ Message string }

func (e *ErrValidation) Error() string { return e.Message }
