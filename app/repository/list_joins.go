package repository

import (
	"strings"

	"gorm.io/gorm"
)

// memberNameResolver is the ExternalJoin resolver for a list's created_by (an SSO user id, no FK to
// anything local): one query per page for the company's members, giving {id, name} — the member's
// name, or their email when SSO hasn't given a name. A user who is no longer a member stays unresolved
// (the list shows "—").
func memberNameResolver(db *gorm.DB, companyID string) func(ids []string) (map[string]interface{}, error) {
	return func(ids []string) (map[string]interface{}, error) {
		var rows []struct {
			UserID string
			Name   string
			Email  string
		}
		if err := db.Table("user_account_sso").Unscoped().
			Select("user_id", "name", "email").
			Where("company_id = ? AND user_id IN ?", companyID, ids).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		out := make(map[string]interface{}, len(rows))
		for _, r := range rows {
			name := strings.TrimSpace(r.Name)
			if name == "" {
				name = r.Email
			}
			out[r.UserID] = map[string]interface{}{"id": r.UserID, "name": name}
		}
		return out, nil
	}
}
