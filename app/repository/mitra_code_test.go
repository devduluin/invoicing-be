package repository

import "testing"

func TestNextMitraSequence(t *testing.T) {
	cases := []struct {
		codes []string
		want  int
	}{
		{nil, 1},
		{[]string{"MTR-0001", "MTR-0009", "MTR-0003"}, 10},
		{[]string{"MTR-12345"}, 12346},
		{[]string{"MTR-", "MTR-ABC", "MTR-0002"}, 3}, // hand-typed codes don't break the sequence
	}
	for _, c := range cases {
		if got := nextMitraSequence(c.codes); got != c.want {
			t.Errorf("nextMitraSequence(%v) = %d, want %d", c.codes, got, c.want)
		}
	}
}

func TestNextSalespersonSequence(t *testing.T) {
	if got := nextPrefixedSequence(salespersonCodePrefix, []string{"SLS-0004", "MTR-0099", "SLS-X"}); got != 5 {
		t.Fatalf("want 5, got %d", got)
	}
}
