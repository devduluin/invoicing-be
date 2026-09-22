package controller

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	companydom "duluin_invoice/app/domain/company"
	membership "duluin_invoice/app/domain/membership"
	"duluin_invoice/app/model"
	"duluin_invoice/app/validation"
	"duluin_invoice/middlewares"
	"duluin_invoice/utils"
)

// The service slices this controller needs.
type CompanyLister interface {
	ListMyCompanies(a membership.Actor) ([]membership.CompanyMembership, error)
}

type CompanyDirectory interface {
	FindByCode(code string) (*model.Company, error)
}

type CompanyController struct {
	lister    CompanyLister
	directory CompanyDirectory
	profile   companydom.ICompanyService
	audit     audit.ILogger
}

func NewCompanyController(
	lister CompanyLister,
	directory CompanyDirectory,
	profile companydom.ICompanyService,
	auditSvc audit.ILogger,
) *CompanyController {
	return &CompanyController{lister: lister, directory: directory, profile: profile, audit: auditSvc}
}

// PUT /api/v1/companies/me — company settings (profile fields + logo upload).
func (ctrl *CompanyController) UpdateMe(c *fiber.Ctx) error {
	var dto companydom.UpdateProfileDTO
	if err := c.BodyParser(&dto); err != nil {
		return utils.BadRequest(c, []string{"Invalid request body"})
	}
	if msgs := validation.Struct(&dto); msgs != nil {
		return utils.ValidationFailed(c, msgs)
	}

	before := middlewares.GetCompany(c)
	company, err := ctrl.profile.UpdateProfile(companydom.Actor{
		UserID:          middlewares.GetUserID(c),
		ActiveCompanyID: middlewares.GetCompanyID(c),
		Token:           c.Get("Authorization"),
	}, dto)
	if err != nil {
		var vErr *companydom.ErrValidation
		if errors.As(err, &vErr) {
			return utils.ValidationFailed(c, []string{err.Error()})
		}
		var nfErr *companydom.ErrCompanyNotFound
		if errors.As(err, &nfErr) {
			return utils.NotFound(c, []string{err.Error()})
		}
		return utils.InternalError(c, err)
	}

	if before != nil {
		changes := companyProfileDiff(before, company)
		if len(changes) > 0 {
			ctrl.audit.Log(auditActor(c), audit.Entry{
				Action:      audit.ActionUpdated,
				Module:      audit.ModuleCompany,
				EntityType:  "company",
				EntityID:    company.ID,
				EntityName:  company.Name,
				Description: "Updated company settings",
				Changes:     changes,
			})
		}
	}
	return utils.Ok(c, company, "Company updated")
}

// companyProfileDiff compares the fields a company owner can actually change from Settings →
// Company, skipping anything that didn't change. company_logo is already a short storage URL by
// the time it reaches here (never the raw uploaded data), so it is safe to record as-is.
func companyProfileDiff(before, after *model.Company) map[string]audit.Change {
	changes := map[string]audit.Change{}
	add := func(field, b, a string) {
		if b != a {
			changes[field] = audit.Change{Before: b, After: a}
		}
	}
	add("name", before.Name, after.Name)
	add("npwp", before.Npwp, after.Npwp)
	add("email", before.Email, after.Email)
	add("phone", before.Phone, after.Phone)
	add("alamat", before.Alamat, after.Alamat)
	add("kota", before.Kota, after.Kota)
	add("provinsi", before.Provinsi, after.Provinsi)
	add("kode_pos", before.KodePos, after.KodePos)
	add("company_logo", before.CompanyLogo, after.CompanyLogo)
	add("jenis_usaha", before.JenisUsaha, after.JenisUsaha)
	add("jumlah_karyawan", before.JumlahKaryawan, after.JumlahKaryawan)
	return changes
}

// GET /api/v1/companies — the companies the user can act in.
func (ctrl *CompanyController) List(c *fiber.Ctx) error {
	companies, err := ctrl.lister.ListMyCompanies(membershipActor(c))
	if err != nil {
		return utils.InternalError(c, err)
	}
	return utils.Ok(c, companies, "OK")
}

// GET /api/v1/companies/me — the active company row.
func (ctrl *CompanyController) Me(c *fiber.Ctx) error {
	company := middlewares.GetCompany(c)
	if company == nil {
		return utils.NotFound(c, []string{"Active company not found"})
	}
	return utils.Ok(c, company, "OK")
}

// companyPublic is the safe-to-share projection returned by the directory
// lookup — enough to pre-fill a Mitra form (PRD §10 network invoicing).
type companyPublic struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	OwnerName string `json:"owner_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Npwp      string `json:"npwp,omitempty"`
	Address   string `json:"address,omitempty"`
}

// GET /api/v1/companies/lookup/:code — resolve a shareable Company ID to
// its public profile. Used to auto-fill a new Mitra. Excludes the caller's own
// active company.
func (ctrl *CompanyController) Lookup(c *fiber.Ctx) error {
	code := strings.TrimSpace(c.Params("code"))
	if code == "" {
		return utils.BadRequest(c, []string{"Company code is required"})
	}

	company, err := ctrl.directory.FindByCode(code)
	if err != nil {
		return utils.InternalError(c, err)
	}
	if company == nil || company.ID == middlewares.GetCompanyID(c) {
		return utils.NotFound(c, []string{"No company found with that ID"})
	}

	return utils.Ok(c, companyPublic{
		ID:        company.ID,
		Code:      company.Code,
		Name:      company.Name,
		OwnerName: company.OwnerName,
		Email:     company.Email,
		Phone:     company.Phone,
		Npwp:      company.Npwp,
		Address:   joinAddress(company),
	}, "OK")
}

func joinAddress(c *model.Company) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{c.Alamat, c.Kota, c.Provinsi, c.KodePos} {
		if s := strings.TrimSpace(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}
