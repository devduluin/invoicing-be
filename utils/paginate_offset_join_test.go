package utils

import "testing"

func TestApplyExternalJoins(t *testing.T) {
	calls := 0
	rows := []map[string]interface{}{
		{"id": "1", "created_by": "u1"},
		{"id": "2", "created_by": "u2"},
		{"id": "3", "created_by": "u1"},
		{"id": "4"},
	}
	applyExternalJoins(rows, []ExternalJoin{{
		Key: "created_by", As: "created_by_rel",
		Resolver: func(ids []string) (map[string]interface{}, error) {
			calls++
			if len(ids) != 2 {
				t.Fatalf("want 2 distinct ids, got %v", ids)
			}
			return map[string]interface{}{"u1": "Budi"}, nil
		},
	}})
	if calls != 1 {
		t.Fatalf("want one batched lookup, got %d", calls)
	}
	if rows[0]["created_by_rel"] != "Budi" || rows[2]["created_by_rel"] != "Budi" {
		t.Fatalf("u1 not resolved: %v", rows)
	}
	if _, ok := rows[1]["created_by_rel"]; ok {
		t.Fatal("an unresolved id must stay empty")
	}
}
