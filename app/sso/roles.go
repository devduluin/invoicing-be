package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Role is one SSO role for this account type. CompanyID nil ⇒ global role shared
// by every company; non-nil ⇒ a company-scoped custom role (SSO prefixes its
// name with the first 8 hex of the company id).
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	CompanyID   *string  `json:"company_id,omitempty"`
	IsCustom    bool     `json:"is_custom"`
	Permissions []string `json:"permissions,omitempty"`
}

// RolePermissions is the per-request resolution of a role id to its permission
// slugs (used by MembershipService.ResolveAccess).
type RolePermissions struct {
	RoleID      string
	RoleName    string
	Permissions []string
}

// RoleDetail powers the (deferred) custom-role editor: the role plus the full
// assignable-permission catalog for this account type.
type RoleDetail struct {
	Role           Role
	AllPermissions []string
}

const rolesDataBase = "/users/rolesData"

// GetRolePermissions resolves one role id to its permission slugs.
// Falls back to the full catalog ONLY when the role itself carries no
// permissions (mirrors acc-master's utils.GetRolePermissions).
func (c *Client) GetRolePermissions(ctx context.Context, roleID, companyID, token string) (RolePermissions, error) {
	if strings.TrimSpace(roleID) == "" {
		return RolePermissions{}, fmt.Errorf("get role permissions: missing role id")
	}
	q := url.Values{}
	q.Set("account_type", c.accountType)
	if s := strings.TrimSpace(companyID); s != "" {
		q.Set("company_id", s)
	}
	path := fmt.Sprintf("%s/roles-by-account/%s?%s", rolesDataBase, url.PathEscape(roleID), q.Encode())

	raw, err := c.do(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return RolePermissions{}, err
	}

	var body struct {
		Data struct {
			Role          rawRole         `json:"role"`
			AllPermission json.RawMessage `json:"all_permission"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return RolePermissions{}, &Error{Path: path, Message: "decode: " + err.Error()}
	}

	perms := namesFrom(body.Data.Role.Permissions)
	if len(perms) == 0 {
		perms = namesFrom(body.Data.AllPermission)
	}
	name := body.Data.Role.Name
	if name == "" {
		name = roleID
	}
	return RolePermissions{RoleID: roleID, RoleName: name, Permissions: perms}, nil
}

// ListRoles returns the global roles plus the custom roles of one company.
// companyID empty ⇒ only the global roles.
func (c *Client) ListRoles(ctx context.Context, companyID, token string) ([]Role, error) {
	q := url.Values{}
	q.Set("account_type", c.accountType)
	q.Set("page", "1")
	q.Set("pageSize", "100")
	if s := strings.TrimSpace(companyID); s != "" {
		q.Set("company_id", s)
	} else {
		q.Set("company_id", "null")
	}
	path := rolesDataBase + "/roles-by-account?" + q.Encode()

	raw, err := c.do(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	return decodeRoleList(path, raw)
}

// GetRoleDetail returns the role with its permissions and the assignable catalog.
func (c *Client) GetRoleDetail(ctx context.Context, roleID, companyID, token string) (RoleDetail, error) {
	if strings.TrimSpace(roleID) == "" {
		return RoleDetail{}, fmt.Errorf("get role detail: missing role id")
	}
	q := url.Values{}
	q.Set("account_type", c.accountType)
	if s := strings.TrimSpace(companyID); s != "" {
		q.Set("company_id", s)
	}
	path := fmt.Sprintf("%s/roles-by-account/%s?%s", rolesDataBase, url.PathEscape(roleID), q.Encode())

	raw, err := c.do(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return RoleDetail{}, err
	}
	var body struct {
		Data struct {
			Role          rawRole         `json:"role"`
			AllPermission json.RawMessage `json:"all_permission"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return RoleDetail{}, &Error{Path: path, Message: "decode: " + err.Error()}
	}
	return RoleDetail{
		Role:           body.Data.Role.toRole(),
		AllPermissions: namesFrom(body.Data.AllPermission),
	}, nil
}

// CreateRole creates a company-scoped custom role in SSO. The returned id is
// authoritative — SSO may prefix the stored name.
func (c *Client) CreateRole(ctx context.Context, companyID, token, name string, permissionNames []string) (Role, error) {
	return c.writeRole(ctx, http.MethodPost, rolesDataBase+"/store-by-account", companyID, token, name, permissionNames)
}

// UpdateRole replaces a custom role's name + permission set.
func (c *Client) UpdateRole(ctx context.Context, roleID, companyID, token, name string, permissionNames []string) (Role, error) {
	if strings.TrimSpace(roleID) == "" {
		return Role{}, fmt.Errorf("update role: missing role id")
	}
	path := fmt.Sprintf("%s/update-by-account/%s", rolesDataBase, url.PathEscape(roleID))
	return c.writeRole(ctx, http.MethodPut, path, companyID, token, name, permissionNames)
}

// DeleteRole removes a custom role by id.
func (c *Client) DeleteRole(ctx context.Context, roleID, token string) error {
	if strings.TrimSpace(roleID) == "" {
		return fmt.Errorf("delete role: missing role id")
	}
	_, err := c.do(ctx, http.MethodPost, rolesDataBase+"/delete", token, map[string]any{"id": roleID})
	return err
}

func (c *Client) writeRole(ctx context.Context, method, path, companyID, token, name string, permissionNames []string) (Role, error) {
	if strings.TrimSpace(name) == "" {
		return Role{}, fmt.Errorf("role name is required")
	}
	payload := map[string]any{
		"name":         name,
		"guard_name":   "web",
		"account_type": c.accountType,
		"permissions":  permissionNames,
	}
	if s := strings.TrimSpace(companyID); s != "" {
		payload["company_id"] = s
	}
	raw, err := c.do(ctx, method, path, token, payload)
	if err != nil {
		return Role{}, err
	}
	var body struct {
		Data rawRole `json:"data"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return Role{}, &Error{Path: path, Message: "decode: " + err.Error()}
	}
	role := body.Data.toRole()
	if role.ID == "" {
		return Role{}, &Error{Path: path, Message: "sso did not return a role id"}
	}
	return role, nil
}

// ── decoding helpers ───────────────────────────────────────────────────────

type rawRole struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	CompanyID   *string         `json:"company_id"`
	Permissions json.RawMessage `json:"permissions"`
}

func (r rawRole) toRole() Role {
	return Role{
		ID:          r.ID,
		Name:        r.Name,
		CompanyID:   r.CompanyID,
		IsCustom:    r.CompanyID != nil && strings.TrimSpace(*r.CompanyID) != "",
		Permissions: namesFrom(r.Permissions),
	}
}

// decodeRoleList tolerates `data` as an array or as `{data:{roles:[...]}}`.
func decodeRoleList(path string, raw []byte) ([]Role, error) {
	var flat struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &flat); err != nil {
		return nil, &Error{Path: path, Message: "decode: " + err.Error()}
	}

	var list []rawRole
	if err := json.Unmarshal(flat.Data, &list); err != nil {
		var wrapped struct {
			Roles []rawRole `json:"roles"`
			Data  []rawRole `json:"data"`
		}
		if err2 := json.Unmarshal(flat.Data, &wrapped); err2 != nil {
			return nil, &Error{Path: path, Message: "decode role list: " + err.Error()}
		}
		list = wrapped.Roles
		if len(list) == 0 {
			list = wrapped.Data
		}
	}

	out := make([]Role, 0, len(list))
	for _, r := range list {
		out = append(out, r.toRole())
	}
	return out, nil
}

// namesFrom pulls permission slugs from either ["a","b"] or [{"name":"a"}].
func namesFrom(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var asStrings []string
	if err := json.Unmarshal(raw, &asStrings); err == nil {
		return dedupeNonEmpty(asStrings)
	}
	var asObjects []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &asObjects); err == nil {
		out := make([]string, 0, len(asObjects))
		for _, o := range asObjects {
			out = append(out, o.Name)
		}
		return dedupeNonEmpty(out)
	}
	return nil
}

func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
