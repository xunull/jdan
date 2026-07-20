package uuidx

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---- 版本 / variant ----

func TestParse_V4(t *testing.T) {
	i, err := Parse("3f2504e0-4f89-41d3-9a0c-0305e82c3301")
	if err != nil {
		t.Fatal(err)
	}
	if i.Version != 4 || i.Variant != "RFC 4122" {
		t.Errorf("got v%d %s", i.Version, i.Variant)
	}
	if i.Timestamp != nil {
		t.Error("v4 should have no timestamp")
	}
}

func TestParse_VersionsFromNibble(t *testing.T) {
	cases := map[string]int{
		"00000000-0000-1000-8000-000000000000": 1,
		"00000000-0000-3000-8000-000000000000": 3,
		"00000000-0000-4000-8000-000000000000": 4,
		"00000000-0000-5000-8000-000000000000": 5,
		"00000000-0000-7000-8000-000000000000": 7,
		"00000000-0000-8000-8000-000000000000": 8,
	}
	for s, want := range cases {
		i, err := Parse(s)
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}
		if i.Version != want {
			t.Errorf("%s: version %d, want %d", s, i.Version, want)
		}
	}
}

func TestParse_Variants(t *testing.T) {
	// byte[8] 的高位决定 variant；用第 9 字节（index 8 = 第三组首字节）控制
	cases := map[string]string{
		"00000000-0000-4000-0000-000000000000": "NCS (兼容旧版)", // 0xxx
		"00000000-0000-4000-8000-000000000000": "RFC 4122",   // 10xx
		"00000000-0000-4000-c000-000000000000": "Microsoft",  // 110x
		"00000000-0000-4000-e000-000000000000": "Reserved",   // 111x
	}
	for s, want := range cases {
		i, _ := Parse(s)
		if i.Variant != want {
			t.Errorf("%s: variant %q, want %q", s, i.Variant, want)
		}
	}
}

// ---- nil / max ----

func TestParse_Nil(t *testing.T) {
	i, _ := Parse("00000000-0000-0000-0000-000000000000")
	if !i.IsNil || i.IsMax {
		t.Errorf("nil flags wrong: nil=%v max=%v", i.IsNil, i.IsMax)
	}
}

func TestParse_Max(t *testing.T) {
	i, _ := Parse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	if !i.IsMax || i.IsNil {
		t.Errorf("max flags wrong: nil=%v max=%v", i.IsNil, i.IsMax)
	}
}

// ---- 时间戳 ----

func TestParse_V7Timestamp(t *testing.T) {
	ms := int64(1719410400000) // 任意已知毫秒
	var b [16]byte
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = 0x70 // version 7
	b[8] = 0x80 // RFC variant
	i, err := Parse(formatCanonical(b))
	if err != nil {
		t.Fatal(err)
	}
	if i.Timestamp == nil || i.Timestamp.UnixMilli() != ms {
		t.Errorf("v7 timestamp = %v, want ms=%d", i.Timestamp, ms)
	}
}

func TestParse_V1Timestamp(t *testing.T) {
	// 取一个已知时间，按 v1 布局编码，再解析回来比对（容许 100ns 量化误差）
	want := time.Date(2024, 6, 26, 14, 0, 0, 0, time.UTC)
	ticks := uint64(want.UnixNano())/100 + gregorianOffset
	var b [16]byte
	timeLow := uint32(ticks & 0xFFFFFFFF)
	timeMid := uint16((ticks >> 32) & 0xFFFF)
	timeHi := uint16((ticks >> 48) & 0x0FFF)
	b[0] = byte(timeLow >> 24)
	b[1] = byte(timeLow >> 16)
	b[2] = byte(timeLow >> 8)
	b[3] = byte(timeLow)
	b[4] = byte(timeMid >> 8)
	b[5] = byte(timeMid)
	b[6] = 0x10 | byte(timeHi>>8) // version 1 + time_hi 高 4 位
	b[7] = byte(timeHi)
	b[8] = 0x80
	i, err := Parse(formatCanonical(b))
	if err != nil {
		t.Fatal(err)
	}
	if i.Timestamp == nil {
		t.Fatal("v1 should have a timestamp")
	}
	if diff := i.Timestamp.Sub(want); diff < -100*time.Nanosecond || diff > 100*time.Nanosecond {
		t.Errorf("v1 timestamp = %v, want %v (diff %v)", i.Timestamp, want, diff)
	}
}

// ---- 输入容错 ----

func TestParse_Tolerant(t *testing.T) {
	want := "3f2504e0-4f89-41d3-9a0c-0305e82c3301"
	for _, in := range []string{
		"3F2504E0-4F89-41D3-9A0C-0305E82C3301", // 大写
		"urn:uuid:3f2504e0-4f89-41d3-9a0c-0305e82c3301",
		"{3f2504e0-4f89-41d3-9a0c-0305e82c3301}",
		"{URN:UUID:3F2504E0-4F89-41D3-9A0C-0305E82C3301}",
		"3f2504e04f8941d39a0c0305e82c3301", // 无连字符
		"  3f2504e0-4f89-41d3-9a0c-0305e82c3301  ",
	} {
		i, err := Parse(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if i.Canonical != want {
			t.Errorf("%q → %s, want %s", in, i.Canonical, want)
		}
	}
}

func TestParse_Errors(t *testing.T) {
	for _, in := range []string{"", "not-a-uuid", "12345", "zzzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("%q should error", in)
		}
	}
}

// ---- 输出 ----

func TestFormatJSON(t *testing.T) {
	i, _ := Parse("00000000-0000-7000-8000-000000000000")
	s, err := i.FormatJSON()
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	if v["version"].(float64) != 7 {
		t.Errorf("json version = %v", v["version"])
	}
	if _, ok := v["time"]; !ok {
		t.Error("v7 JSON should include time")
	}
}

func TestFormatText(t *testing.T) {
	i, _ := Parse("3f2504e0-4f89-41d3-9a0c-0305e82c3301")
	out := i.FormatText()
	for _, want := range []string{"canonical:", "version:   4", "variant:   RFC 4122", "urn:uuid:"} {
		if !strings.Contains(out, want) {
			t.Errorf("text missing %q:\n%s", want, out)
		}
	}
}
