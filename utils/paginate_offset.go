package utils

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// OffsetPaginationOptions drives GetPaginatedDataOffset. Ported from
// acc-master-service/utils/getPaginatedDataOffset.go (cross-service row joins
// dropped — invoice-service has no external relation resolvers yet).
type OffsetPaginationOptions struct {
	Model                interface{}
	Where                map[string]interface{}
	Limit                int
	Page                 int
	Sort                 string
	Order                string
	Select               []string
	Exclude              []string
	Preloads             []string
	ValidColumns         []string
	PreserveAssociations bool // true → fetch into the model struct then JSON-encode (keeps preloads/BoolInt)
	SelectPrefix         string
	// PreloadRelations — like Preloads, but only the listed columns of the related row, and soft-deleted
	// related rows included (a document keeps showing a partner that was deleted since).
	PreloadRelations []PreloadRelation
	// KeepColumns are always selected, even when the caller narrows the columns (?fields=) — the
	// foreign keys PreloadRelations / ExternalJoins need to resolve.
	KeepColumns []string
	// ExternalJoins fill a row field from data a GORM relation can't reach (another table keyed by a
	// non-FK value, or another service): one batched lookup per page, never one per row.
	ExternalJoins []ExternalJoin
	// DetailPreloads are loaded only when WithDetails (a list asked with ?with=details, e.g. an export):
	// a document's lines, a partner's contact persons. Kept off the normal list so it stays light.
	DetailPreloads []PreloadRelation
	WithDetails    bool
}

// PreloadRelation — a GORM association to preload with a narrow column list.
type PreloadRelation struct {
	Name    string   // association field, e.g. "Mitra"
	Columns []string // e.g. {"id", "code", "name"}; empty = all
	// Scoped keeps soft-deleted related rows out (a partner's deleted contact persons); by default they
	// are included (a document still shows the partner it was made for after that partner was deleted).
	Scoped bool
	Order  string // e.g. "line_order ASC"
}

// ExternalJoin puts Resolver(distinct values of row[Key])[value] into row[As]. A resolver error leaves
// the field empty rather than failing the list.
type ExternalJoin struct {
	Key      string // row field holding the lookup value, e.g. "created_by"
	As       string // field the resolved value is written to, e.g. "created_by_rel"
	Resolver func(values []string) (map[string]interface{}, error)
}

type OffsetPaginationMeta struct {
	TotalItems  int64 `json:"totalItems"`
	TotalPages  int   `json:"totalPages"`
	CurrentPage int   `json:"currentPage"`
	PerPage     int   `json:"perPage"`
	HasNextPage bool  `json:"hasNextPage"`
	HasPrevPage bool  `json:"hasPrevPage"`
}

type OffsetPaginationResult struct {
	Data       []map[string]interface{} `json:"data"`
	Columns    []string                 `json:"columns"`
	Attributes []string                 `json:"attributes"`
	Meta       OffsetPaginationMeta      `json:"meta"`
}

// GetPaginatedDataOffset runs a COUNT + a single paginated SELECT against the
// (already-scoped) query and returns the rows as maps plus the column catalog
// the frontend's column-visibility menu needs.
func GetPaginatedDataOffset(db *gorm.DB, options OffsetPaginationOptions) (*OffsetPaginationResult, error) {
	if options.Model != nil {
		db = db.Model(options.Model)
	}
	if len(options.Where) > 0 {
		db = db.Where(options.Where)
	}
	for _, preload := range options.Preloads {
		if strings.TrimSpace(preload) == "" {
			continue
		}
		db = db.Preload(preload)
	}
	rels := options.PreloadRelations
	if options.WithDetails {
		rels = append(append([]PreloadRelation{}, rels...), options.DetailPreloads...)
	}
	for _, rel := range rels {
		rel := rel
		db = db.Preload(rel.Name, func(tx *gorm.DB) *gorm.DB {
			if !rel.Scoped {
				tx = tx.Unscoped()
			}
			if len(rel.Columns) > 0 {
				tx = tx.Select(rel.Columns)
			}
			if rel.Order != "" {
				tx = tx.Order(rel.Order)
			}
			return tx
		})
	}

	safePage := normalizePositive(options.Page, 1)
	safeLimit := normalizePositive(options.Limit, 10)
	offset := (safePage - 1) * safeLimit

	selectedColumns := sanitizeAttributes(options.Select, options.Exclude, options.ValidColumns)
	if len(selectedColumns) > 0 {
		query := selectedColumns
		for _, k := range options.KeepColumns {
			if !containsString(query, k) {
				query = append(append([]string{}, query...), k)
			}
		}
		db = db.Select(qualifySelectColumns(query, options.SelectPrefix))
	} else if len(options.Exclude) > 0 {
		db = db.Omit(options.Exclude...)
	}

	sortColumn := strings.TrimSpace(options.Sort)
	if sortColumn == "" {
		sortColumn = "created_at"
	}
	orderDirection := strings.ToUpper(strings.TrimSpace(options.Order))
	if orderDirection != "ASC" && orderDirection != "DESC" {
		orderDirection = "DESC"
	}

	var total int64
	countDB := db.Session(&gorm.Session{})
	if err := countDB.Count(&total).Error; err != nil {
		return nil, err
	}

	orderClause := fmt.Sprintf("%s %s", sortColumn, orderDirection)
	rows := make([]map[string]interface{}, 0)

	if options.PreserveAssociations && options.Model != nil {
		modelType := reflect.TypeOf(options.Model)
		if modelType.Kind() == reflect.Ptr {
			modelType = modelType.Elem()
		}
		sliceType := reflect.SliceOf(modelType)
		slicePtr := reflect.New(sliceType)

		if err := db.Order(orderClause).Offset(offset).Limit(safeLimit).Find(slicePtr.Interface()).Error; err != nil {
			return nil, err
		}
		sliceVal := slicePtr.Elem()
		rows = make([]map[string]interface{}, sliceVal.Len())
		for i := 0; i < sliceVal.Len(); i++ {
			jsonBytes, _ := json.Marshal(sliceVal.Index(i).Interface())
			_ = json.Unmarshal(jsonBytes, &rows[i])
		}
	} else {
		if err := db.Order(orderClause).Offset(offset).Limit(safeLimit).Find(&rows).Error; err != nil {
			return nil, err
		}
	}

	columns := filterColumns(options.ValidColumns, options.Exclude)
	attributes := columns
	if len(selectedColumns) > 0 {
		attributes = selectedColumns
	} else if len(attributes) == 0 && len(options.Select) > 0 {
		attributes = options.Select
	}

	if rows == nil {
		rows = []map[string]interface{}{}
	}
	applyExternalJoins(rows, options.ExternalJoins)

	totalPages := int(math.Ceil(float64(total) / float64(safeLimit)))
	return &OffsetPaginationResult{
		Data:       rows,
		Columns:    columns,
		Attributes: attributes,
		Meta: OffsetPaginationMeta{
			TotalItems:  total,
			TotalPages:  totalPages,
			CurrentPage: safePage,
			PerPage:     safeLimit,
			HasNextPage: safePage < totalPages,
			HasPrevPage: safePage > 1,
		},
	}, nil
}

func normalizePositive(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// BuildOffsetMeta computes the pagination meta for a list that was assembled in
// memory (enriched views etc.) rather than through GetPaginatedDataOffset.
func BuildOffsetMeta(total int64, page, limit int) OffsetPaginationMeta {
	page = normalizePositive(page, 1)
	limit = normalizePositive(limit, 10)
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	return OffsetPaginationMeta{
		TotalItems:  total,
		TotalPages:  totalPages,
		CurrentPage: page,
		PerPage:     limit,
		HasNextPage: page < totalPages,
		HasPrevPage: page > 1,
	}
}

// ToJSONMaps marshals any slice to []map[string]interface{} using its JSON tags.
func ToJSONMaps(v interface{}) []map[string]interface{} {
	raw, err := json.Marshal(v)
	if err != nil {
		return []map[string]interface{}{}
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil || out == nil {
		return []map[string]interface{}{}
	}
	return out
}

// qualifySelectColumns prefixes bare columns with tablePrefix so a joined query
// stays unambiguous. No-op when tablePrefix is empty or a column is already qualified.
func qualifySelectColumns(columns []string, tablePrefix string) []string {
	if tablePrefix == "" {
		return columns
	}
	qualified := make([]string, len(columns))
	for i, col := range columns {
		if strings.Contains(col, ".") {
			qualified[i] = col
		} else {
			qualified[i] = tablePrefix + "." + col
		}
	}
	return qualified
}

func sanitizeAttributes(attributes, exclude, validColumns []string) []string {
	if len(attributes) == 0 {
		return nil
	}
	excludeSet := toSet(exclude)
	validSet := toSet(validColumns)

	var sanitized []string
	for _, attr := range attributes {
		trimmed := strings.TrimSpace(attr)
		if trimmed == "" {
			continue
		}
		if _, excluded := excludeSet[trimmed]; excluded {
			continue
		}
		if len(validSet) > 0 {
			if _, ok := validSet[trimmed]; !ok {
				continue
			}
		}
		sanitized = append(sanitized, trimmed)
	}
	return sanitized
}

func filterColumns(validColumns, exclude []string) []string {
	if len(validColumns) == 0 {
		return nil
	}
	excludeSet := toSet(exclude)
	var filtered []string
	for _, col := range validColumns {
		if col == "" {
			continue
		}
		if _, excluded := excludeSet[col]; excluded {
			continue
		}
		filtered = append(filtered, col)
	}
	return filtered
}

func toSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

// ParseCSVParam splits a comma-separated query param ("name,kind,rate") into a
// trimmed, non-empty slice.
func ParseCSVParam(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// NormalizeSort maps a requested sort field (camelCase or snake_case) to a
// whitelisted column, falling back to `fallback` when it isn't allowed.
func NormalizeSort(sort string, validColumns []string, fallback string) string {
	trimmed := toSnakeCase(strings.TrimSpace(sort))
	if trimmed == "" {
		return fallback
	}
	for _, col := range validColumns {
		if col == trimmed {
			return trimmed
		}
	}
	return fallback
}

func toSnakeCase(s string) string {
	if s == "" || !strings.ContainsAny(s, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// applyExternalJoins resolves each ExternalJoin once for the whole page.
func applyExternalJoins(rows []map[string]interface{}, joins []ExternalJoin) {
	for _, j := range joins {
		if j.Resolver == nil || j.Key == "" || j.As == "" {
			continue
		}
		seen := map[string]bool{}
		values := make([]string, 0, len(rows))
		for _, r := range rows {
			if v, ok := r[j.Key].(string); ok && v != "" && !seen[v] {
				seen[v] = true
				values = append(values, v)
			}
		}
		if len(values) == 0 {
			continue
		}
		resolved, err := j.Resolver(values)
		if err != nil {
			continue
		}
		for _, r := range rows {
			if v, ok := r[j.Key].(string); ok {
				if val, found := resolved[v]; found {
					r[j.As] = val
				}
			}
		}
	}
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
