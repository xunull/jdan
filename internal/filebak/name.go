package filebak

import (
	"strings"
	"unicode"
)

// NormalizeDesc trims surrounding space. Empty after trim means "no description" (ok=true, "").
// Otherwise each rune must be ASCII letter, ASCII digit, Han, or ASCII space; otherwise ok=false.
// On success, spaces are replaced with underscores.
func NormalizeDesc(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", true
	}
	for _, r := range s {
		switch {
		case r == ' ':
			continue
		case r >= 'a' && r <= 'z':
			continue
		case r >= 'A' && r <= 'Z':
			continue
		case r >= '0' && r <= '9':
			continue
		case unicode.Is(unicode.Han, r):
			continue
		default:
			return "", false
		}
	}
	return strings.ReplaceAll(s, " ", "_"), true
}
