package handler

import (
	"strings"
	"unicode"
)

// Client headers sent by the mobile app (TEC-1179, docs/design/mobile.md §2.1). The web BFF
// builds its upstream headers from an allowlist and never forwards them, so browser sessions
// are always "web". X-Client only picks the refresh lifetime and labels of a NEW session
// family; rotation keeps the family's stored type.
const (
	headerClient     = "X-Client"
	headerDeviceName = "X-Device-Name"
	headerAppVersion = "X-App-Version"

	maxDeviceNameLen = 100
	maxAppVersionLen = 32
)

// isMobileClient accepts "mobile" and the app's full identifier "comcode-mobile".
func isMobileClient(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "mobile", "comcode-mobile":
		return true
	}
	return false
}

// clientLabel keeps printable characters only (no control or format characters), collapses
// whitespace and caps the length in runes. Empty when nothing usable is left.
func clientLabel(v string, maxLen int) string {
	var b strings.Builder
	n := 0
	space := false
	for _, r := range strings.TrimSpace(v) {
		if n >= maxLen {
			break
		}
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if !unicode.IsPrint(r) {
			continue
		}
		if space && n > 0 {
			b.WriteByte(' ')
			n++
			if n >= maxLen {
				break
			}
		}
		space = false
		b.WriteRune(r)
		n++
	}
	return b.String()
}
