package filebak

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// ErrInvalidDesc means --desc contained characters other than ASCII letters, digits, Han, or ASCII space.
var ErrInvalidDesc = errors.New("无效描述：仅允许英文字母、汉字、数字与空格")

// BackupDestination returns the backup file path next to src. descRaw is the raw --desc flag value
// (may be empty); it is validated and normalized the same way as NormalizeDesc.
func BackupDestination(src string, now time.Time, descRaw string) (string, error) {
	descNorm, ok := NormalizeDesc(descRaw)
	if !ok {
		return "", ErrInvalidDesc
	}
	src = filepath.Clean(src)
	dir := filepath.Dir(src)
	base := filepath.Base(src)
	ts := now.Format("20060102-150405")
	var name string
	if descNorm == "" {
		name = fmt.Sprintf("%s.bak.%s", base, ts)
	} else {
		name = fmt.Sprintf("%s.bak.%s-%s", base, ts, descNorm)
	}
	return filepath.Join(dir, name), nil
}

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
