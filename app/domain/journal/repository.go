package domain_journal

import (
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

type IRepository interface {
	Create(dto *CreateDTO, actorID string) (*model.JournalEntry, error)
	Update(companyID, id string, dto *UpdateDTO, actorID string) (*model.JournalEntry, error)
	FindByID(companyID, id string) (*model.JournalEntry, error)
	FindAll(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	SetStatus(companyID, id, actorID string, status model.JournalEntryStatus) error
	// AccountsActive reports whether every given account id is active
	// (part of accounting-engine-service parity: a journal line can't post
	// against a deactivated account).
	AccountsActive(companyID string, accountIDs []string) (bool, error)
	// FindByIdempotencyKey returns the existing entry for a repeated create
	// request, or nil if none exists yet.
	FindByIdempotencyKey(companyID, key string) (*model.JournalEntry, error)
}

type IService interface {
	Create(companyID, actorID string, dto *CreateDTO) (*model.JournalEntry, error)
	Update(companyID, actorID, id string, dto *UpdateDTO) (*model.JournalEntry, error)
	Get(companyID, id string) (*model.JournalEntry, error)
	List(f *Filter) (*utils.OffsetPaginationResult, error)
	Delete(companyID, id string) error
	Post(companyID, actorID, id string) (*model.JournalEntry, error)
	BackToDraft(companyID, actorID, id string) (*model.JournalEntry, error)
}
