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

// Appearance — the ONE colour a document is themed with. Empty = the template's own palette.
type Appearance struct {
	Color string `json:"color,omitempty"` // "#rrggbb"
}

// Margins are in millimetres; nil = the built-in default for that side.
type Margins struct {
	Top    *float64 `json:"top,omitempty"`
	Bottom *float64 `json:"bottom,omitempty"`
	Left   *float64 `json:"left,omitempty"`
	Right  *float64 `json:"right,omitempty"`
}

// PageLayout — paper size, orientation and margins the PDF (and every preview) is laid out on.
type PageLayout struct {
	Size        string  `json:"size,omitempty"`        // "a4" | "a5" | "letter"
	Orientation string  `json:"orientation,omitempty"` // "portrait" | "landscape"
	Margins     Margins `json:"margins"`
}

// HeaderFooter — whole-header / logo / accent-line switches and the PDF page footer.
type HeaderFooter struct {
	ShowHeader     *bool `json:"showHeader,omitempty"`
	ShowLogo       *bool `json:"showLogo,omitempty"`
	AccentLine     *bool `json:"accentLine,omitempty"`
	ShowFooter     *bool `json:"showFooter,omitempty"`
	ShowPageNumber *bool `json:"showPageNumber,omitempty"`
}

// TextStyle — an override of one text group; every field optional (unset = the template's own).
type TextStyle struct {
	Font      string   `json:"font,omitempty"` // "inter" | "sans" | "serif" | "mono"
	Size      *float64 `json:"size,omitempty"` // pt
	Color     string   `json:"color,omitempty"`
	Bold      *bool    `json:"bold,omitempty"`
	Italic    *bool    `json:"italic,omitempty"`
	Underline *bool    `json:"underline,omitempty"`
	Align     string   `json:"align,omitempty"` // "left" | "center" | "right"
}

// TemplateStyle — the look of ONE template of ONE document type. Every field is optional; a
// template with nothing stored keeps its own built-in look.
type TemplateStyle struct {
	Appearance Appearance           `json:"appearance"`
	Page       PageLayout           `json:"page"`
	Header     HeaderFooter         `json:"header"`
	TextStyles map[string]TextStyle `json:"textStyles,omitempty"`
}

// Formats — how numbers, money, dates, tax and discounts are printed.
type Formats struct {
	Number   string `json:"number,omitempty"`   // "id" (1.000,00) | "en" (1,000.00) | "space" (1 000,00)
	Decimals *int   `json:"decimals,omitempty"` // 0..4
	Currency string `json:"currency,omitempty"` // "rp" | "idr" | "usd" | "none"
	Date     string `json:"date,omitempty"`     // "dmy" | "dmy-dash" | "ymd" | "long"
	Tax      string `json:"tax,omitempty"`      // "name" | "rate"
	Discount string `json:"discount,omitempty"` // "entered" | "percent" | "amount"
}

// Config is the shared schema for every document type.
type Config struct {
	Language     string               `json:"language,omitempty"` // "id" | "en"
	DocumentName string               `json:"documentName,omitempty"`
	Labels       map[string]string    `json:"labels,omitempty"`
	Hidden       []string             `json:"hidden,omitempty"`
	Shown        []string             `json:"shown,omitempty"`
	ColumnOrder  []string             `json:"columnOrder,omitempty"`
	Notes        Block                `json:"notes"`
	Terms        Block                `json:"terms"`
	Signature    Signature            `json:"signature"`
	// TemplateStyles is keyed by template id ("template_1".."template_7"), or "default" for the
	// document types that have no templates (receipts, delivery note, goods receipt).
	TemplateStyles map[string]TemplateStyle `json:"templateStyles,omitempty"`
	Formats        Formats                  `json:"formats"`
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
