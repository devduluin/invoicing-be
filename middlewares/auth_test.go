package middlewares

import "testing"

func TestRoleMatches(t *testing.T) {
	cases := []struct {
		name      string
		userRole  string
		required  string
		companyID string
		want      bool
	}{
		{"exact", "Invoice Owner", "Invoice Owner", "", true},
		{"company scoped", "abc-Invoice Admin", "Invoice Admin", "abc", true},
		{"company scoped mismatch", "xyz-Invoice Admin", "Invoice Admin", "abc", false},
		{"company scoped no context", "xyz-Invoice Admin", "Invoice Admin", "", true},
		{"different role", "Invoice Viewer", "Invoice Admin", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := roleMatches(tc.userRole, tc.required, tc.companyID); got != tc.want {
				t.Fatalf("roleMatches(%q,%q,%q) = %v, want %v", tc.userRole, tc.required, tc.companyID, got, tc.want)
			}
		})
	}
}

func TestToBool(t *testing.T) {
	for _, tc := range []struct {
		in   interface{}
		want bool
	}{
		{true, true},
		{float64(1), true},
		{float64(0), false},
		{"1", true},
		{"true", true},
		{"0", false},
		{nil, false},
	} {
		if got := toBool(tc.in); got != tc.want {
			t.Fatalf("toBool(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
