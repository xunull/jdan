package dotenv

import "strings"

// RedactOpts 控制脱敏策略。
type RedactOpts struct {
	Full      bool // 完全打码（****），不保留首尾
	KeepShort bool // 短值（<=4）/ 布尔类不打码
}

// RedactValue 脱敏单个 value。
//   - 空值 → 空
//   - Full → 固定 "****"
//   - KeepShort 且值短/布尔 → 原样返回
//   - 默认 → 保留首尾各 1-2 字符，中间用 * 填充
func RedactValue(v string, opts RedactOpts) string {
	if v == "" {
		return ""
	}
	if opts.KeepShort && (len(v) <= 4 || isBoolish(v)) {
		return v
	}
	if opts.Full {
		return "****"
	}
	n := len(v)
	switch {
	case n <= 2:
		// 太短，无法保留首尾还隐藏 → 全打码
		return strings.Repeat("*", n)
	case n <= 6:
		// 保留首尾各 1
		return v[:1] + strings.Repeat("*", n-2) + v[n-1:]
	default:
		// 保留首尾各 2
		return v[:2] + strings.Repeat("*", n-4) + v[n-2:]
	}
}

func isBoolish(v string) bool {
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "0", "1":
		return true
	}
	return false
}

// RedactLine 渲染脱敏后的 KEY=value 行（保留 export 前缀）。
func RedactLine(e Entry, opts RedactOpts) string {
	if !e.HasEquals {
		return e.Raw
	}
	var sb strings.Builder
	if e.HadExport {
		sb.WriteString("export ")
	}
	sb.WriteString(e.Key)
	sb.WriteByte('=')
	sb.WriteString(RedactValue(e.Value, opts))
	return sb.String()
}
