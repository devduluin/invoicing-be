// Package audit is the reusable audit-trail port: one Actor + one Entry in, one append-only row out.
// Any service or controller that performs a significant action calls Log once instead of writing its
// own logging — the schema, the company isolation and the "never edited, never deleted" guarantee
// live here, in one place.
package audit

import (
	"encoding/json"
	"time"

	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// Canonical action vocabulary — also what the list/filter API accepts.
const (
	ActionCreated            = "created"
	ActionUpdated            = "updated"
	ActionDeleted            = "deleted"
	ActionRestored           = "restored"
	ActionApproved           = "approved"
	ActionRejected           = "rejected"
	ActionSubmitted          = "submitted"
	ActionCancelled          = "cancelled"
	ActionSent               = "sent"
	ActionPayment            = "payment"
	ActionStatusChanged      = "status_changed"
	ActionLogin              = "login"
	ActionLogout             = "logout"
	ActionInvitedUser        = "invited_user"
	ActionAcceptedInvitation = "accepted_invitation"
	ActionRemovedUser        = "removed_user"
	ActionRoleChanged        = "role_changed"
	ActionPermissionChanged  = "permission_changed"
	ActionOther              = "other"
)

// Canonical module vocabulary.
const (
	ModuleCompany         = "company"
	ModuleUserManagement  = "user_management"
	ModuleRoleManagement  = "role_management"
	ModuleSalesOrder      = "sales_order"
	ModuleDownPayment     = "down_payment"
	ModuleSalesInvoice    = "sales_invoice"
	ModuleSalesReceipt    = "sales_receipt"
	ModuleDeliveryNote    = "delivery_note"
	ModulePurchaseOrder   = "purchase_order"
	ModulePurchaseInvoice = "purchase_invoice"
	ModulePurchaseReceipt = "purchase_receipt"
	ModuleGoodsReceipt    = "goods_receipt"
	ModuleSettings        = "settings"
	ModuleOther           = "other"
)

// Actor is who performed the action and from where. Built by the controller layer from the request
// (JWT claims, IP, User-Agent) — this package and its service never depend on the web framework.
type Actor struct {
	CompanyID string
	UserID    string
	Name      string
	Email     string
	IPAddress string
	UserAgent string
}

// Change is one field's before/after value, JSON-encodable as-is (string, number, bool, nil, …).
type Change struct {
	Before any `json:"before"`
	After  any `json:"after"`
}

// Entry is one action to record. Action, Module and Description are always expected; the Entity*
// fields apply when the action concerns one specific record (a document, a user, …), and Changes
// only when the action changed field values worth diffing.
type Entry struct {
	Action      string
	Module      string
	EntityType  string
	EntityID    string
	EntityName  string // document number, or the target person's name — whichever applies
	Description string
	Changes     map[string]Change
}

// View is one row as the API returns it (list and detail share this shape).
type View struct {
	ID          string          `json:"id"`
	ActorUserID string          `json:"actor_user_id,omitempty"`
	ActorName   string          `json:"actor_name"`
	ActorEmail  string          `json:"actor_email,omitempty"`
	Action      string          `json:"action"`
	Module      string          `json:"module"`
	EntityType  string          `json:"entity_type,omitempty"`
	EntityID    string          `json:"entity_id,omitempty"`
	EntityName  string          `json:"entity_name,omitempty"`
	Description string          `json:"description"`
	Changes     json.RawMessage `json:"changes,omitempty"`
	IPAddress   string          `json:"ip_address,omitempty"`
	UserAgent   string          `json:"user_agent,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// Filter drives the paginated, company-scoped audit list. CompanyID is always required by the
// repository — there is no "all companies" query anywhere in this package.
type Filter struct {
	CompanyID string
	Search    string
	Actions   []string
	Modules   []string
	UserID    string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

// IRepository is append-only on purpose: no Update, no Delete, no method that could touch an
// existing row's actor/action/timestamp. That is the whole enforcement mechanism for "audit logs
// can't be edited or deleted" — the capability simply does not exist in the type system.
type IRepository interface {
	Insert(rec *model.AuditLog) error
	List(f Filter) ([]model.AuditLog, int64, error)
	FindByID(companyID, id string) (*model.AuditLog, error)
}

// ILogger is the narrow port most services/controllers depend on — just "record this happened".
type ILogger interface {
	Log(a Actor, e Entry)
}

// IService is the full port the Audit Log page's controller depends on: log, list, read one.
type IService interface {
	ILogger
	List(f Filter) (*utils.OffsetPaginationResult, error)
	Get(companyID, id string) (*View, error)
}
