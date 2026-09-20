package utils

import (
	"html"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// Notes and Terms & Conditions are edited with TipTap, which emits HTML. Older
// rows hold plain text (newlines only). Both live in the same text column:
// content that starts with a TipTap block tag is rich text and is sanitized
// here; anything else stays plain text (the frontend escapes it on render).
// The frontend applies the SAME start-of-string rule (lib/richText.ts) — keep
// the two in sync.
var richTextStart = regexp.MustCompile(`(?i)^\s*<(p|ul|ol|br)[\s/>]`)

var htmlTag = regexp.MustCompile(`<[^>]*>`)

var richTextPolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	// Exactly what the editor can produce: paragraphs, line breaks, bold /
	// italic / underline / strike, lists, links and text alignment. Nothing else
	// (no headings, images, tables, scripts, classes, ids, inline styles other
	// than text-align).
	p.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s", "ul", "ol", "li")
	p.AllowStyles("text-align").MatchingEnum("left", "center", "right", "justify").OnElements("p", "li")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// LooksLikeRichText reports whether s is TipTap-style HTML (vs legacy plain text).
func LooksLikeRichText(s string) bool { return richTextStart.MatchString(s) }

// SanitizeRichText trims s and, when it is rich text, strips everything outside
// the allow-list above. Legacy plain text passes through untouched. An editor
// that was emptied out (`<p></p>`) collapses to "".
func SanitizeRichText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || !LooksLikeRichText(s) {
		return s
	}
	out := strings.TrimSpace(richTextPolicy.Sanitize(s))
	if strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(out, ""))) == "" {
		return ""
	}
	return out
}
