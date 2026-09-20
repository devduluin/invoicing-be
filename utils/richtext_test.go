package utils

import (
	"strings"
	"testing"
)

func TestSanitizeRichText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // exact, or "" with contains/notContains below
	}{
		{"empty", "  ", ""},
		{"emptied editor", "<p></p>", ""},
		{"only whitespace paragraphs", "<p> </p><p></p>", ""},
		{"plain text untouched", "Line 1\nLine 2 & <b>not html</b>", "Line 1\nLine 2 & <b>not html</b>"},
		{"formatting kept", "<p>a <strong>b</strong> <em>c</em> <u>d</u></p>", "<p>a <strong>b</strong> <em>c</em> <u>d</u></p>"},
		{"lists kept", "<ul><li>x</li></ul><ol><li>y</li></ol>", "<ul><li>x</li></ul><ol><li>y</li></ol>"},
		{"alignment kept", `<p style="text-align: center">x</p>`, `<p style="text-align: center">x</p>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeRichText(tc.in); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestSanitizeRichText_StripsDangerousMarkup(t *testing.T) {
	bad := map[string]string{
		"script":        `<p>hi</p><script>alert(1)</script>`,
		"onerror":       `<p><img src=x onerror=alert(1)></p>`,
		"onclick":       `<p onclick="alert(1)">x</p>`,
		"js href":       `<p><a href="javascript:alert(1)">x</a></p>`,
		"data href":     `<p><a href="data:text/html;base64,PHNjcmlwdD4=">x</a></p>`,
		"iframe":        `<p>x</p><iframe src="https://evil"></iframe>`,
		"style expr":    `<p style="background:url(javascript:alert(1))">x</p>`,
		"svg":           `<p><svg onload=alert(1)></svg></p>`,
		"class and id":  `<p class="a" id="b">x</p>`,
		"nested script": `<ul><li><script>alert(1)</script>ok</li></ul>`,
	}
	for name, in := range bad {
		out := SanitizeRichText(in)
		for _, needle := range []string{"<script", "onerror", "onclick", "javascript:", "data:", "<iframe", "<svg", "onload", "url(", `class=`, `id=`} {
			if strings.Contains(strings.ToLower(out), needle) {
				t.Errorf("%s: %q survived in %q", name, needle, out)
			}
		}
	}
}

func TestSanitizeRichText_Links(t *testing.T) {
	out := SanitizeRichText(`<p><a href="https://example.com/x">site</a> <a href="mailto:a@b.co">mail</a></p>`)
	if !strings.Contains(out, `href="https://example.com/x"`) || !strings.Contains(out, `href="mailto:a@b.co"`) {
		t.Fatalf("safe links dropped: %q", out)
	}
	if !strings.Contains(out, `rel="nofollow noreferrer noopener"`) && !strings.Contains(out, "noopener") {
		t.Fatalf("external link missing rel protection: %q", out)
	}
	if !strings.Contains(out, `target="_blank"`) {
		t.Fatalf("external link should open in a new tab: %q", out)
	}
}

func TestLooksLikeRichText(t *testing.T) {
	for in, want := range map[string]bool{
		"<p>x</p>": true, " \n<ul><li>x</li></ul>": true, "<OL><li>x</li></OL>": true,
		"hello <p>": false, "1 < 2": false, "<script>": false, "": false,
	} {
		if LooksLikeRichText(in) != want {
			t.Errorf("LooksLikeRichText(%q) != %v", in, want)
		}
	}
}
