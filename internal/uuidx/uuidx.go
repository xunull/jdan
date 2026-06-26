// Package uuidx 解析/检视 UUID（RFC 9562 / 4122）：版本、variant、v1/v7 内嵌时间戳、
// 字节、nil/max。只做解析，不做生成（生成在 internal/randgen，jdan rand uuid）。
package uuidx

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Info 是解析后的 UUID 信息。
type Info struct {
	Canonical string
	Version   int
	Variant   string
	Bytes     [16]byte
	Timestamp *time.Time // 仅 v1 / v7
	IsNil     bool
	IsMax     bool
}

// gregorianOffset 是 1582-10-15 到 1970-01-01 之间的 100ns 间隔数（v1 时间戳用）。
const gregorianOffset = 122192928000000000

func normalize(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, "{}")
	s = strings.TrimPrefix(s, "urn:uuid:")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) != 32 {
		return "", fmt.Errorf("UUID 应为 32 个 hex 字符（去掉连字符后），实际 %d", len(s))
	}
	return s, nil
}

// Parse 解析一个 UUID 字符串。容错：urn:uuid: 前缀 / {花括号} / 无连字符 / 大小写。
func Parse(s string) (Info, error) {
	hexStr, err := normalize(s)
	if err != nil {
		return Info{}, err
	}
	raw, err := hex.DecodeString(hexStr)
	if err != nil {
		return Info{}, fmt.Errorf("含非 hex 字符: %w", err)
	}
	var b [16]byte
	copy(b[:], raw)

	info := Info{
		Bytes:     b,
		Canonical: formatCanonical(b),
		Version:   int(b[6] >> 4),
		Variant:   variantOf(b[8]),
		IsNil:     isAll(b, 0x00),
		IsMax:     isAll(b, 0xFF),
	}
	switch info.Version {
	case 7:
		t := timestampV7(b)
		info.Timestamp = &t
	case 1:
		t := timestampV1(b)
		info.Timestamp = &t
	}
	return info, nil
}

func formatCanonical(b [16]byte) string {
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst)
}

// variantOf 看 byte[8] 的高位判定 variant（RFC 9562 §4.1）。
func variantOf(b byte) string {
	switch {
	case b&0x80 == 0x00: // 0xxx
		return "NCS (兼容旧版)"
	case b&0xC0 == 0x80: // 10xx
		return "RFC 4122"
	case b&0xE0 == 0xC0: // 110x
		return "Microsoft"
	default: // 111x
		return "Reserved"
	}
}

func isAll(b [16]byte, v byte) bool {
	for _, x := range b {
		if x != v {
			return false
		}
	}
	return true
}

// timestampV7 取前 48 bit 当 unix 毫秒。
func timestampV7(b [16]byte) time.Time {
	ms := int64(b[0])<<40 | int64(b[1])<<32 | int64(b[2])<<24 |
		int64(b[3])<<16 | int64(b[4])<<8 | int64(b[5])
	return time.UnixMilli(ms).UTC()
}

// timestampV1 取 60-bit 100ns 计数（自 1582-10-15），转 unix 时间。
func timestampV1(b [16]byte) time.Time {
	timeLow := uint64(b[0])<<24 | uint64(b[1])<<16 | uint64(b[2])<<8 | uint64(b[3])
	timeMid := uint64(b[4])<<8 | uint64(b[5])
	timeHi := uint64(b[6]&0x0F)<<8 | uint64(b[7])
	ticks := timeHi<<48 | timeMid<<32 | timeLow
	ns := (int64(ticks) - gregorianOffset) * 100
	return time.Unix(0, ns).UTC()
}

func versionNote(v int) string {
	switch v {
	case 1:
		return " (时间 + 节点)"
	case 4:
		return " (随机)"
	case 7:
		return " (时间排序)"
	case 3, 5:
		return " (名字 hash)"
	case 8:
		return " (自定义)"
	default:
		return ""
	}
}

func hexBytes(b [16]byte) string {
	parts := make([]string, 16)
	for i, x := range b {
		parts[i] = fmt.Sprintf("%02x", x)
	}
	return strings.Join(parts, " ")
}

// FormatText 渲染成文本。
func (i Info) FormatText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "canonical: %s\n", i.Canonical)
	switch {
	case i.IsNil:
		b.WriteString("type:      Nil UUID (全 0)\n")
	case i.IsMax:
		b.WriteString("type:      Max UUID (全 F)\n")
	}
	fmt.Fprintf(&b, "version:   %d%s\n", i.Version, versionNote(i.Version))
	fmt.Fprintf(&b, "variant:   %s\n", i.Variant)
	if i.Timestamp != nil {
		fmt.Fprintf(&b, "time:      %s\n", i.Timestamp.Format("2006-01-02 15:04:05.000 UTC"))
	}
	fmt.Fprintf(&b, "bytes:     %s\n", hexBytes(i.Bytes))
	fmt.Fprintf(&b, "urn:       urn:uuid:%s\n", i.Canonical)
	return b.String()
}

// FormatJSON 渲染成结构化输出。
func (i Info) FormatJSON() (string, error) {
	out := map[string]any{
		"canonical": i.Canonical,
		"version":   i.Version,
		"variant":   i.Variant,
		"nil":       i.IsNil,
		"max":       i.IsMax,
	}
	if i.Timestamp != nil {
		out["time"] = i.Timestamp.Format(time.RFC3339Nano)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
