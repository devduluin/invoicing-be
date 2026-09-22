package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"duluin_invoice/utils"
)

// AccountType — PRD §2. Captured in onboarding Step 2 (Informasi Perusahaan).
type AccountType string

const (
	AccountTypePerseorangan AccountType = "perseorangan"
	AccountTypeEnterprise   AccountType = "enterprise"
)

func IsValidAccountType(v string) bool {
	return AccountType(v) == AccountTypePerseorangan || AccountType(v) == AccountTypeEnterprise
}

// EmployeeCountBuckets — the "Company size" dropdown options (Free-plan lead-signal taxonomy).
// Stored verbatim as a string.
var EmployeeCountBuckets = []string{"1-10", "11-50", "51-100", "100+"}

func IsValidEmployeeCount(v string) bool {
	for _, b := range EmployeeCountBuckets {
		if b == v {
			return true
		}
	}
	return false
}

// OnboardingStatus on companies — mirrors acc-master. A company is created
// `active` in one shot (the wizard is a client-side draft committed once); the
// column stays for a future "created but not verified" state.
type OnboardingStatus string

const (
	OnboardingPending OnboardingStatus = "pending"
	OnboardingActive  OnboardingStatus = "active"
)

// FreeInviteQuota — PRD §4 "Undang 9 Orang Gratis". The company owner does not count.
const FreeInviteQuota = 9

// DefaultFreeTransactionLimitIDR — PRD §2/§5 placeholder column only.
const DefaultFreeTransactionLimitIDR = 1_000_000

// Company is a Duluin Invoice workspace. Its own UUID is the canonical
// company_id used by every business table and is mirrored to the SSO account as
// user_accounts.secondary_id (acc-master pattern). Membership + role live in
// user_account_sso.
type Company struct {
	ID   string `gorm:"type:uuid;primaryKey"    json:"id"`
	Name string `gorm:"type:varchar(255)"       json:"name"`
	// Code — the short, random, shareable Company ID (Paper.id equivalent).
	// Used for network-invoicing partner lookups (PRD §10). Unique, not derived
	// from the name.
	Code string `gorm:"type:varchar(20);index" json:"code"`

	// OwnerName — snapshot of the creator's name at onboarding, used to pre-fill
	// the "Nama Kontak" when another company adds this one as a Mitra (PRD §10).
	OwnerName string `gorm:"type:varchar(255)" json:"owner_name,omitempty"`

	// CompanyLogo — public URL of the uploaded logo (SSO Minio storage), shown on
	// invoices/documents (PRD §4 "Setup Template & Logo"). Same upload path as
	// acc-master (sso.Client.UploadBlob).
	CompanyLogo string `gorm:"type:varchar(512)" json:"company_logo,omitempty"`

	Email    string `gorm:"type:varchar(150)" json:"email,omitempty"`
	Phone    string `gorm:"type:varchar(50)"  json:"phone,omitempty"`
	Npwp     string `gorm:"type:varchar(50)"  json:"npwp,omitempty"`
	Alamat   string `gorm:"type:varchar(255)" json:"alamat,omitempty"`
	Kota     string `gorm:"type:varchar(100)" json:"kota,omitempty"`
	Provinsi string `gorm:"type:varchar(100)" json:"provinsi,omitempty"`
	KodePos  string `gorm:"type:varchar(20)"  json:"kode_pos,omitempty"`

	OnboardingStatus OnboardingStatus `gorm:"type:varchar(20);not null;default:'pending'" json:"onboarding_status"`

	// Step 2 — flat columns (no jsonb blob). Address parts live in the
	// Email/Phone/Npwp/Alamat/Kota/Provinsi/KodePos fields above.
	TipeAkun       *AccountType `gorm:"type:varchar(20)"  json:"tipe_akun,omitempty"`
	JenisUsaha     string       `gorm:"type:varchar(120)" json:"jenis_usaha,omitempty"`
	JumlahKaryawan string       `gorm:"type:varchar(20)"  json:"jumlah_karyawan,omitempty"` // bucket, e.g. "6-10"

	// Step 3 — selected need slugs, comma-joined (kept as a scalar, not an array).
	KebutuhanUser string `gorm:"type:text" json:"kebutuhan_user,omitempty"`

	// Lead signal for the Duluin ecosystem — derived from KebutuhanUser at onboarding submit (and
	// whenever it's changed): does this company say they already need HR/Payroll/Accounting/
	// Attendance help? Used for the contextual "Explore Duluin" CTA, never a hard block.
	HasHRSystem         bool `gorm:"not null;default:false" json:"has_hr_system"`
	HasPayrollSystem    bool `gorm:"not null;default:false" json:"has_payroll_system"`
	HasAccountingSystem bool `gorm:"not null;default:false" json:"has_accounting_system"`
	HasAttendanceSystem bool `gorm:"not null;default:false" json:"has_attendance_system"`

	// Verification flags — PRD §4/§5 (Stage 2 OTP / doc review).
	EmailVerified utils.BoolInt `gorm:"type:smallint;not null;default:0" json:"email_verified"`
	PhoneVerified utils.BoolInt `gorm:"type:smallint;not null;default:0" json:"phone_verified"`

	// TODO(stage-2): cumulative-transaction limit (PRD §5). Column not read yet.
	FreeTransactionLimitIDR int64 `gorm:"column:free_transaction_limit_idr;type:bigint;not null;default:1000000" json:"free_transaction_limit_idr"`

	// Website — optional, collected for the Duluin lead ecosystem alongside the rest of the company
	// profile. Duluin Invoice has no Product/Service master, so there is nothing product-related here.
	Website string `gorm:"type:varchar(255)" json:"website,omitempty"`

	// ActivationStatus — "initial" (Free workspace, tighter limits) until the three-step activation
	// milestone is met (company profile + 3 partners + 1 invoice), then "activated" (Full Free
	// limits) permanently — it is never re-locked by later deleting a partner or invoice back below
	// the threshold. See app/service/activation.service.go for the computation.
	ActivationStatus string     `gorm:"type:varchar(20);not null;default:'initial'" json:"activation_status"`
	ActivatedAt      *time.Time `gorm:"column:activated_at" json:"activated_at,omitempty"`

	CreatedAt time.Time      `gorm:"autoCreateTime"   json:"created_at"`
	CreatedBy string         `gorm:"type:varchar(64)" json:"created_by,omitempty"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"   json:"updated_at"`
	UpdatedBy string         `gorm:"type:varchar(64)" json:"updated_by,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index"            json:"-"`
}

func (Company) TableName() string { return "companies" }

func (c *Company) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

func (c *Company) OnboardingComplete() bool { return c.OnboardingStatus == OnboardingActive }

// ApplyLeadSignals recomputes the has_*_system flags from KebutuhanUser (the Step 3 "What do you
// need help with?" selection — see lib/onboarding.ts NEED_OPTIONS on the frontend). Call this
// whenever KebutuhanUser is set or changed.
func (c *Company) ApplyLeadSignals() {
	needs := strings.Split(c.KebutuhanUser, ",")
	has := func(slug string) bool {
		for _, n := range needs {
			if strings.TrimSpace(n) == slug {
				return true
			}
		}
		return false
	}
	c.HasHRSystem = has("hr_management")
	c.HasPayrollSystem = has("payroll")
	c.HasAccountingSystem = has("accounting")
	c.HasAttendanceSystem = has("attendance")
}

// TransactionLimitLifted — PRD §4: both channels verified → cap fully open.
func (c *Company) TransactionLimitLifted() bool {
	return bool(c.EmailVerified) && bool(c.PhoneVerified)
}
