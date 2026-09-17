package service

// defaultsSeeder is the shared shape of a per-company master-data seeder.
type defaultsSeeder interface {
	SeedDefaults(companyID, actorID string) error
}

// MasterDefaultsService seeds the template COA + taxes when a company is created
// (PRD §7 / §9). Each underlying seeder is idempotent (no-op when rows exist).
type MasterDefaultsService struct {
	accounts defaultsSeeder
	taxes    defaultsSeeder
}

func NewMasterDefaultsService(accounts, taxes defaultsSeeder) *MasterDefaultsService {
	return &MasterDefaultsService{accounts: accounts, taxes: taxes}
}

func (s *MasterDefaultsService) SeedCompanyDefaults(companyID, actorID string) error {
	if err := s.accounts.SeedDefaults(companyID, actorID); err != nil {
		return err
	}
	return s.taxes.SeedDefaults(companyID, actorID)
}
