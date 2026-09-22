package service

import (
	"encoding/json"
	"log"
	"strings"

	domain "duluin_invoice/app/domain/audit"
	"duluin_invoice/app/model"
	"duluin_invoice/utils"
)

// AuditService is the one reusable audit-trail mechanism every other service/controller calls
// instead of writing its own logging. Log is deliberately fire-and-forget from the caller's point of
// view: a failed audit write is logged and swallowed, never returned as an error — recording an
// action must never be the reason the action itself fails.
type AuditService struct {
	repo domain.IRepository
}

func NewAuditService(repo domain.IRepository) *AuditService {
	return &AuditService{repo: repo}
}

var _ domain.IService = (*AuditService)(nil)

func (s *AuditService) Log(a domain.Actor, e domain.Entry) {
	companyID := strings.TrimSpace(a.CompanyID)
	if companyID == "" || e.Action == "" || e.Module == "" {
		// No company to scope the row to, or a caller that forgot the required fields — refuse
		// silently rather than write a row that could never be found again.
		log.Printf("[audit] dropped incomplete entry: company=%q action=%q module=%q", companyID, e.Action, e.Module)
		return
	}

	var changesJSON string
	if len(e.Changes) > 0 {
		b, err := json.Marshal(e.Changes)
		if err != nil {
			log.Printf("[audit] failed to encode changes: %v", err)
		} else {
			changesJSON = string(b)
		}
	}

	rec := &model.AuditLog{
		CompanyID:   companyID,
		ActorUserID: strings.TrimSpace(a.UserID),
		ActorName:   strings.TrimSpace(a.Name),
		ActorEmail:  strings.ToLower(strings.TrimSpace(a.Email)),
		Action:      e.Action,
		Module:      e.Module,
		EntityType:  e.EntityType,
		EntityID:    e.EntityID,
		EntityName:  e.EntityName,
		Description: strings.TrimSpace(e.Description),
		Changes:     changesJSON,
		IPAddress:   a.IPAddress,
		UserAgent:   a.UserAgent,
	}
	if rec.ActorName == "" {
		rec.ActorName = rec.ActorEmail
	}
	if err := s.repo.Insert(rec); err != nil {
		log.Printf("[audit] insert failed action=%s module=%s company=%s: %v", e.Action, e.Module, companyID, err)
	}
}

func (s *AuditService) List(f domain.Filter) (*utils.OffsetPaginationResult, error) {
	rows, total, err := s.repo.List(f)
	if err != nil {
		return nil, err
	}
	out := make([]domain.View, 0, len(rows))
	for i := range rows {
		out = append(out, toAuditView(&rows[i]))
	}
	cols := []string{"created_at", "actor_name", "action", "module", "description", "entity_name", "ip_address"}
	return &utils.OffsetPaginationResult{
		Data:       utils.ToJSONMaps(out),
		Columns:    cols,
		Attributes: cols,
		Meta:       utils.BuildOffsetMeta(total, f.Page, f.PageSize),
	}, nil
}

func (s *AuditService) Get(companyID, id string) (*domain.View, error) {
	rec, err := s.repo.FindByID(companyID, id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	v := toAuditView(rec)
	return &v, nil
}

func toAuditView(rec *model.AuditLog) domain.View {
	return domain.View{
		ID:          rec.ID,
		ActorUserID: rec.ActorUserID,
		ActorName:   rec.ActorName,
		ActorEmail:  rec.ActorEmail,
		Action:      rec.Action,
		Module:      rec.Module,
		EntityType:  rec.EntityType,
		EntityID:    rec.EntityID,
		EntityName:  rec.EntityName,
		Description: rec.Description,
		Changes:     rawOrNil(rec.Changes),
		IPAddress:   rec.IPAddress,
		UserAgent:   rec.UserAgent,
		CreatedAt:   rec.CreatedAt,
	}
}

func rawOrNil(s string) json.RawMessage {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return json.RawMessage(s)
}
