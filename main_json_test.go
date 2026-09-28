package main

import (
	"strings"
	"testing"
)

func TestMarshalJSONWithoutHTMLEscaping(t *testing.T) {
	b, err := marshalJSONWithoutHTMLEscaping(map[string]string{
		"invite_url": "https://invoicing-dev.duluin.id/accept-invite?email=a@b.com&name=A&token=X",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "\\u0026") {
		t.Fatalf("ampersand was HTML-escaped: %s", s)
	}
	if !strings.Contains(s, "email=a@b.com&name=A&token=X") {
		t.Fatalf("real ampersands missing: %s", s)
	}
}
