package controller

import (
	"strings"
	"testing"
)

type auditDoc struct {
	ID        string   `json:"id"`
	Number    string   `json:"number"`
	Notes     string   `json:"notes"`
	Total     float64  `json:"grand_total"`
	Lines     []string `json:"lines"`
	Signature string   `json:"signature_data"`
	UpdatedAt string   `json:"updated_at"`
}

func TestDocDiff(t *testing.T) {
	before := &auditDoc{ID: "1", Number: "SO/1", Notes: "a", Total: 10, Lines: []string{"x"}, Signature: "s", UpdatedAt: "t1"}
	after := &auditDoc{ID: "1", Number: "SO/1", Notes: "b", Total: 25, Lines: []string{"x", "y"}, Signature: strings.Repeat("z", 5000), UpdatedAt: "t2"}

	got := docDiff(before, after)
	if _, noisy := got["updated_at"]; noisy {
		t.Error("updated_at must not be reported")
	}
	if _, same := got["number"]; same {
		t.Error("unchanged fields must not be reported")
	}
	if got["notes"].Before != "a" || got["notes"].After != "b" {
		t.Errorf("notes diff wrong: %+v", got["notes"])
	}
	if got["lines"].Before != "1 item(s)" || got["lines"].After != "2 item(s)" {
		t.Errorf("lines should be summarised by count: %+v", got["lines"])
	}
	if got["signature_data"].After != "(changed)" {
		t.Errorf("huge values must not enter the trail: %+v", got["signature_data"])
	}
	if docDiff(nil, after) != nil || docDiff((*auditDoc)(nil), after) != nil {
		t.Error("a missing 'before' yields no diff")
	}
}

func TestNestedFieldDiff(t *testing.T) {
	before := []byte(`{"language":"id","templateStyles":{"template_4":{"appearance":{"color":"#111111"}}},"hidden":["a"]}`)
	after := []byte(`{"language":"id","templateStyles":{"template_4":{"appearance":{"color":"#16A34A"},"page":{"orientation":"landscape"}}},"hidden":["a","b"]}`)
	got := nestedFieldDiff(before, after)
	if c := got["templateStyles.template_4.appearance.color"]; c.Before != "#111111" || c.After != "#16A34A" {
		t.Errorf("colour change wrong: %+v", c)
	}
	if c := got["templateStyles.template_4.page.orientation"]; c.Before != nil || c.After != "landscape" {
		t.Errorf("added leaf wrong: %+v", c)
	}
	if _, ok := got["language"]; ok {
		t.Error("unchanged leaf reported")
	}
	if _, ok := got["hidden"]; !ok {
		t.Error("changed list not reported")
	}
}
