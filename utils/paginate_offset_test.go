package utils

import (
	"reflect"
	"testing"
)

func TestNormalizeSort(t *testing.T) {
	cols := []string{"name", "created_at", "is_active"}
	cases := []struct{ in, want string }{
		{"", "code"},
		{"name", "name"},
		{"createdAt", "created_at"},
		{"isActive", "is_active"},
		{"id", "code"},         // not whitelisted → fallback
		{"name; DROP", "code"}, // junk → fallback
	}
	for _, c := range cases {
		if got := NormalizeSort(c.in, cols, "code"); got != c.want {
			t.Errorf("NormalizeSort(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseCSVParam(t *testing.T) {
	got := ParseCSVParam(" name , kind ,, rate ")
	want := []string{"name", "kind", "rate"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseCSVParam = %v, want %v", got, want)
	}
	if ParseCSVParam("  ") != nil {
		t.Error("blank param should be nil")
	}
}

func TestBuildOffsetMeta(t *testing.T) {
	m := BuildOffsetMeta(45, 2, 20)
	if m.TotalItems != 45 || m.TotalPages != 3 || m.CurrentPage != 2 || m.PerPage != 20 {
		t.Fatalf("bad meta: %+v", m)
	}
	if !m.HasNextPage || !m.HasPrevPage {
		t.Fatalf("page 2 of 3 should have next+prev: %+v", m)
	}
	if BuildOffsetMeta(0, 1, 20).HasNextPage {
		t.Error("empty list should not have a next page")
	}
}

func TestToJSONMaps(t *testing.T) {
	type row struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}
	got := ToJSONMaps([]row{{"a", 1}, {"b", 2}})
	if len(got) != 2 || got[0]["name"] != "a" || got[1]["n"] != float64(2) {
		t.Fatalf("bad maps: %v", got)
	}
	if len(ToJSONMaps([]row{})) != 0 {
		t.Error("empty slice → empty maps")
	}
}

func TestSanitizeAttributes(t *testing.T) {
	valid := []string{"name", "kind", "rate"}
	got := sanitizeAttributes([]string{"name", "secret", " rate "}, []string{"kind"}, valid)
	want := []string{"name", "rate"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sanitizeAttributes = %v, want %v", got, want)
	}
}
