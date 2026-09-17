package domain_mitra

// CreateMitraDTO is the create payload for a Mitra (PRD §8).
type CreateMitraDTO struct {
	CompanyID   string `json:"-"`
	Type        string `json:"type" validate:"required,oneof=customer supplier both"`
	Name        string `json:"name" validate:"required,max=255"`
	ContactName string `json:"contact_name" validate:"omitempty,max=255"`
	Email       string `json:"email" validate:"omitempty,email,max=150"`
	Phone       string `json:"phone" validate:"omitempty,max=50"`
	Npwp        string `json:"npwp" validate:"omitempty,max=50"`
	Address     string `json:"address" validate:"omitempty"`
	IsActive    *bool  `json:"is_active"`
}

// UpdateMitraDTO — all fields optional; only provided keys are applied.
type UpdateMitraDTO struct {
	Type        string  `json:"type" validate:"omitempty,oneof=customer supplier both"`
	Name        string  `json:"name" validate:"omitempty,max=255"`
	ContactName string  `json:"contact_name" validate:"omitempty,max=255"`
	Email       string  `json:"email" validate:"omitempty,email,max=150"`
	Phone       string  `json:"phone" validate:"omitempty,max=50"`
	Npwp        string  `json:"npwp" validate:"omitempty,max=50"`
	Address     *string `json:"address"`
	IsActive    *bool   `json:"is_active"`
}

// MitraFilter drives the list query.
type MitraFilter struct {
	CompanyID string
	Search    string
	Type      string
	IsActive  *bool
	Page      int
	PageSize  int
	Sort      string
	Order     string
	Fields    []string
}
