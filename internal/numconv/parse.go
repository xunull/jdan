// Package numconv 实现 jdan num 命令的核心：进制自动检测 + uint64 解析 +
// dec/hex/bin/oct 互转 + 位展示 + 二元/单目位运算。全部基于 stdlib（strconv +
// math/bits），0 新依赖。
//
// 范围用 uint64（覆盖绝大多数寄存器 / 权限位 / flag mask 场景）；负数和超
// uint64 的值返回清晰错误，不静默 wrap。
package numconv

import (
	"fmt"
	"strconv"
	"strings"
)

// Base 是检测到的输入进制。
type Base int

const (
	BaseDec Base = 10
	BaseHex Base = 16
	BaseBin Base = 2
	BaseOct Base = 8
)

func (b Base) String() string {
	switch b {
	case BaseHex:
		return "hex"
	case BaseBin:
		return "binary"
	case BaseOct:
		return "octal"
	default:
		return "decimal"
	}
}

// DetectBase 按前缀判断输入进制（不解析值）：
//   - 0x / 0X → hex
//   - 0b / 0B → binary
//   - 0o / 0O → octal
//   - 前导 0 且后面还有数字（如 0755）→ octal（C 风格）
//   - 其余 → decimal
//
// 返回 (base, 去掉前缀后的数字部分)。
func DetectBase(s string) (Base, string) {
	s = strings.TrimSpace(s)
	switch {
	case len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X'):
		return BaseHex, s[2:]
	case len(s) >= 2 && s[0] == '0' && (s[1] == 'b' || s[1] == 'B'):
		return BaseBin, s[2:]
	case len(s) >= 2 && s[0] == '0' && (s[1] == 'o' || s[1] == 'O'):
		return BaseOct, s[2:]
	case len(s) >= 2 && s[0] == '0' && isAllDigits(s[1:]):
		// 0755 这种 C 风格八进制
		return BaseOct, s[1:]
	default:
		return BaseDec, s
	}
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ParseValue 自动检测进制并解析成 uint64。
//   - 负号 / 超 uint64 范围 → 清晰错误
//   - 接受下划线分隔符（如 0xFF_FF，跟 Go 字面量一致）
func ParseValue(s string) (uint64, Base, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "-") {
		return 0, 0, fmt.Errorf("negative numbers not supported (uint64 only): %q", s)
	}
	base, digits := DetectBase(s)
	digits = strings.ReplaceAll(digits, "_", "")
	if digits == "" {
		return 0, 0, fmt.Errorf("no digits after base prefix: %q", s)
	}
	v, err := strconv.ParseUint(digits, int(base), 64)
	if err != nil {
		// 区分溢出和非法字符
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return 0, 0, fmt.Errorf("value %q exceeds uint64 range (max 18446744073709551615)", s)
		}
		return 0, 0, fmt.Errorf("invalid %s number %q", base, s)
	}
	return v, base, nil
}
