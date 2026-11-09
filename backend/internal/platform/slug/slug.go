package slug

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// turkishFold maps Turkish-specific letters (both cases) to their ASCII
// equivalents. Without it the ASCII-only slug regex strips these letters, so
// "Çağrı Şirketi" collapsed to "a-r-irketi". Folding before strings.ToLower
// also pins "İ" (U+0130) and "ı" (U+0131, which has no decomposition) to a
// plain "i", independent of ToLower's case mapping.
var turkishFold = strings.NewReplacer(
	"İ", "i", "I", "i", "ı", "i",
	"Ş", "s", "ş", "s",
	"Ğ", "g", "ğ", "g",
	"Ç", "c", "ç", "c",
	"Ö", "o", "ö", "o",
	"Ü", "u", "ü", "u",
)

// stripMarks decomposes (NFD) and drops combining marks so other Latin
// diacritics keep their base letter ("Café" → "cafe") instead of being stripped.
func stripMarks(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		return s
	}
	return out
}

// FromName turns a display name into a URL-safe slug (ASCII-oriented).
// Only non-ASCII input is affected by the folding; ASCII slugs are unchanged
// and stored slugs are never rewritten.
func FromName(name string) string {
	name = turkishFold.Replace(strings.TrimSpace(name))
	name = strings.ToLower(stripMarks(name))
	if name == "" {
		return "item"
	}
	slug := nonSlug.ReplaceAllString(name, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "item"
	}
	if len(slug) > 80 {
		slug = strings.Trim(slug[:80], "-")
	}
	return slug
}
