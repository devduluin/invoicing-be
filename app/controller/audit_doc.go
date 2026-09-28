package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
)

// documentAudit records the audit-trail entries of one kind of record (a document type, a partner…).
// Every controller that changes data owns one, so "who did what, when" is captured the same way for
// all of them instead of only for the ones that happened to call the logger.
type documentAudit struct {
	log        audit.ILogger
	module     string
	entityType string
}

// record writes one entry. A nil logger or an empty id records nothing (nothing to point at).
func (d documentAudit) record(c *fiber.Ctx, action, id, name, description string, changes map[string]audit.Change) {
	if d.log == nil || id == "" {
		return
	}
	d.log.Log(auditActor(c), audit.Entry{
		Action:      action,
		Module:      d.module,
		EntityType:  d.entityType,
		EntityID:    id,
		EntityName:  name,
		Description: description,
		Changes:     changes,
	})
}

// statusChange is the entry for a document moving between draft / confirmed / cancelled.
func (d documentAudit) statusChange(c *fiber.Ctx, action, id, name, verb, before, after string) {
	change := audit.Change{Before: nil, After: after}
	if before != "" {
		change.Before = before
	}
	d.record(c, action, id, name, fmt.Sprintf("%s %s", verb, name), map[string]audit.Change{"status": change})
}

// noisyAuditFields change on every save (or are derived) and say nothing about what the user did.
var noisyAuditFields = map[string]bool{
	"updated_at": true, "updated_by": true, "created_at": true, "created_by": true, "deleted_at": true,
	"id": true, "company_id": true,
}

// maxAuditValueLen keeps a huge value (a signature image, a long rich-text body) out of the trail.
const maxAuditValueLen = 300

// docDiff lists the fields whose value differs between two versions of a record, by their JSON names.
// Line items are summarised by count (their amounts show up in the totals). It is a diagnostic diff,
// not a semantic one: a field appears only when its serialized value changed.
func docDiff(before, after any) map[string]audit.Change {
	if before == nil || after == nil {
		return nil
	}
	bm, am := toJSONMap(before), toJSONMap(after)
	if bm == nil || am == nil {
		return nil
	}
	changes := map[string]audit.Change{}
	keys := map[string]bool{}
	for k := range bm {
		keys[k] = true
	}
	for k := range am {
		keys[k] = true
	}
	for k := range keys {
		if noisyAuditFields[k] {
			continue
		}
		bv, av := bm[k], am[k]
		if string(bv) == string(av) {
			continue
		}
		changes[k] = audit.Change{Before: auditValue(k, bv), After: auditValue(k, av)}
	}
	return changes
}

func toJSONMap(v any) map[string]json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}

func auditValue(key string, r json.RawMessage) any {
	if len(r) == 0 {
		return nil
	}
	if key == "lines" || key == "allocations" || key == "contact_persons" {
		var items []json.RawMessage
		if json.Unmarshal(r, &items) == nil {
			return fmt.Sprintf("%d item(s)", len(items))
		}
	}
	if len(r) > maxAuditValueLen {
		return "(changed)"
	}
	return rawJSONValue(r)
}

// justCreatedWindow is how soon after creating a document its confirmation still counts as part of
// the same "Save & Confirm" action rather than a separate one.
const justCreatedWindow = 15 * time.Second

// confirmsJustCreated reports whether this confirmation is the second half of the create the same
// user just did (the form's "Save & Confirm" creates, then confirms). The trail then keeps just the
// "Created" entry instead of a Created + Confirmed pair for what the user did in one click.
func confirmsJustCreated(c *fiber.Ctx, createdAt time.Time, createdBy string) bool {
	actor := auditActor(c)
	return createdBy != "" && createdBy == actor.UserID && time.Since(createdAt) < justCreatedWindow
}
