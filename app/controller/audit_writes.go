package controller

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"

	audit "duluin_invoice/app/domain/audit"
	"duluin_invoice/middlewares"
)

// AuditSpec describes how one resource's writes appear in the audit trail. AuditWrites turns it into
// a middleware, so every master-data / accounting / access resource is recorded the same way without
// each controller repeating the logging code.
type AuditSpec struct {
	Module string
	Entity string // entity_type stored on the entry, e.g. "account"
	Label  string // how the description names it, e.g. "account"
	// Match limits the middleware to the requests it should record (nil = every write). Used where
	// a controller already records some of its own actions.
	Match func(method, path string) bool
	// Lookup returns the current stored record for the id in the URL (nil when unknown). It runs
	// BEFORE the handler, so an update can show what changed and a delete can still name what was deleted.
	// ids are every uuid in the URL, outermost first; the last one is the record itself.
	Lookup func(c *fiber.Ctx, companyID string, ids []string) any
}

var uuidInPath = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// statusSegments are the action sub-routes (POST /:id/<segment>) and how they read in the trail.
var statusSegments = map[string]string{
	"post": "Posted", "draft": "Moved back to draft:", "confirm": "Confirmed", "cancel": "Cancelled",
	"verify": "Verified", "resend": "Resent invitation to",
}

func auditWritesAction(method, path string) (action, verb string, ok bool) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	last := segs[len(segs)-1]
	switch method {
	case http.MethodPost:
		if last == "invite" {
			return audit.ActionInvitedUser, "Invited", true
		}
		if v, isStatus := statusSegments[last]; isStatus {
			if last == "cancel" {
				return audit.ActionCancelled, v, true
			}
			return audit.ActionStatusChanged, v, true
		}
		return audit.ActionCreated, "Created", true
	case http.MethodPut, http.MethodPatch:
		switch last {
		case "template":
			return audit.ActionUpdated, "Changed the template of", true
		case "assignments":
			return audit.ActionUpdated, "Updated the assignments of", true
		case "role":
			return audit.ActionRoleChanged, "Changed the role of", true
		}
		return audit.ActionUpdated, "Updated", true
	case http.MethodDelete:
		return audit.ActionDeleted, "Deleted", true
	}
	return "", "", false
}

// displayNameFromJSON builds a human-readable name for a record from its JSON form.
func displayNameFromJSON(m map[string]json.RawMessage) string {
	str := func(k string) string {
		var s string
		if raw, ok := m[k]; ok && json.Unmarshal(raw, &s) == nil {
			return strings.TrimSpace(s)
		}
		return ""
	}
	if raw, ok := m["role"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(raw, &nested) == nil {
			if n := displayNameFromJSON(nested); n != "" {
				return n
			}
		}
	}
	switch {
	case str("number") != "":
		return str("number")
	case str("code") != "" && str("name") != "":
		return str("code") + " " + str("name")
	case str("name") != "":
		return str("name")
	case str("bank_name") != "":
		return strings.TrimSpace(str("bank_name") + " " + str("account_number"))
	case str("doc_type") != "":
		return str("doc_type")
	case str("email") != "":
		return str("email")
	}
	return ""
}

// AuditWrites records every successful write (POST / PUT / PATCH / DELETE) under the route it wraps.
// A request that fails (validation, permission, business rule) records nothing: only what actually
// happened is in the trail.
func AuditWrites(l audit.ILogger, spec AuditSpec) fiber.Handler {
	return func(c *fiber.Ctx) error {
		method := c.Method()
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			return c.Next()
		}
		path := c.Path()
		if spec.Match != nil && !spec.Match(method, path) {
			return c.Next()
		}
		action, verb, ok := auditWritesAction(method, path)
		if !ok {
			return c.Next()
		}

		id := ""
		ids := uuidInPath.FindAllString(path, -1)
		if len(ids) > 0 {
			id = ids[len(ids)-1]
		}
		var before any
		// A create has no record yet (its URL id, if any, belongs to a parent such as the partner).
		if id != "" && spec.Lookup != nil && action != audit.ActionCreated {
			before = spec.Lookup(c, middlewares.GetCompanyID(c), ids)
		}

		if err := c.Next(); err != nil {
			return err
		}
		if code := c.Response().StatusCode(); code < 200 || code >= 300 {
			return nil
		}

		// What the handler returned: {"data": {...}} for create / update / status changes.
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		var after map[string]json.RawMessage
		if json.Unmarshal(c.Response().Body(), &envelope) == nil && len(envelope.Data) > 0 {
			_ = json.Unmarshal(envelope.Data, &after)
		}

		name := ""
		if after != nil {
			name = displayNameFromJSON(after)
			if rid := rawString(after["id"]); rid != "" {
				id = rid
			}
		}
		if name == "" && before != nil {
			if bm := toJSONMap(before); bm != nil {
				name = displayNameFromJSON(bm)
			}
		}
		if name == "" {
			segs := strings.Split(strings.Trim(path, "/"), "/")
			last := segs[len(segs)-1]
			if _, isStatus := statusSegments[last]; id == "" && !isStatus {
				name = last // e.g. a document type: /document-templates/sales_invoice
			}
		}

		var changes map[string]audit.Change
		switch {
		case (action == audit.ActionUpdated || action == audit.ActionRoleChanged) && before != nil && after != nil:
			changes = diffMaps(toJSONMap(before), after)
		case action == audit.ActionStatusChanged || action == audit.ActionCancelled:
			if st := rawString(after["status"]); st != "" {
				changes = map[string]audit.Change{"status": {Before: statusBefore(before), After: st}}
			}
		}

		description := strings.TrimSpace(verb + " " + spec.Label + " " + name)
		l.Log(auditActor(c), audit.Entry{
			Action:      action,
			Module:      spec.Module,
			EntityType:  spec.Entity,
			EntityID:    id,
			EntityName:  name,
			Description: description,
			Changes:     changes,
		})
		return nil
	}
}

func rawString(r json.RawMessage) string {
	var s string
	if len(r) > 0 && json.Unmarshal(r, &s) == nil {
		return s
	}
	return ""
}

func statusBefore(before any) any {
	if before == nil {
		return nil
	}
	if bm := toJSONMap(before); bm != nil {
		if s := rawString(bm["status"]); s != "" {
			return s
		}
	}
	return nil
}

// diffMaps is docDiff for two already-decoded records.
func diffMaps(bm, am map[string]json.RawMessage) map[string]audit.Change {
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
		if noisyAuditFields[k] || string(bm[k]) == string(am[k]) {
			continue
		}
		changes[k] = audit.Change{Before: auditValue(k, bm[k]), After: auditValue(k, am[k])}
	}
	return changes
}

// nestedFieldDiff compares two JSON objects and reports each changed LEAF by its dotted path
// (e.g. "templateStyles.template_4.appearance.color"), so a settings change reads as the one thing
// that changed instead of a whole nested blob. Arrays and long values are summarised.
func nestedFieldDiff(beforeJSON, afterJSON []byte) map[string]audit.Change {
	var b, a any
	_ = json.Unmarshal(beforeJSON, &b)
	_ = json.Unmarshal(afterJSON, &a)
	changes := map[string]audit.Change{}
	walkDiff("", b, a, changes)
	return changes
}

func walkDiff(path string, b, a any, out map[string]audit.Change) {
	bm, bok := b.(map[string]any)
	am, aok := a.(map[string]any)
	if bok || aok {
		keys := map[string]bool{}
		for k := range bm {
			keys[k] = true
		}
		for k := range am {
			keys[k] = true
		}
		for k := range keys {
			next := k
			if path != "" {
				next = path + "." + k
			}
			walkDiff(next, bm[k], am[k], out)
		}
		return
	}
	bj, _ := json.Marshal(b)
	aj, _ := json.Marshal(a)
	if string(bj) == string(aj) || path == "" {
		return
	}
	out[path] = audit.Change{Before: auditValue(path, bj), After: auditValue(path, aj)}
}
