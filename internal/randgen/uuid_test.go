package randgen

import (
	crypto_rand "crypto/rand"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// 8-4-4-4-12 canonical format check helper
func checkUUIDStructure(t *testing.T, s string) {
	t.Helper()
	if len(s) != 36 {
		t.Fatalf("UUID length = %d, want 36 (got %q)", len(s), s)
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		t.Fatalf("not 5 dash-separated parts: %q", s)
	}
	wantLens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != wantLens[i] {
			t.Errorf("part %d len = %d, want %d", i, len(p), wantLens[i])
		}
		for _, c := range p {
			if !strings.ContainsRune("0123456789abcdef", c) {
				t.Errorf("non-hex char %q in part %d", c, i)
			}
		}
	}
}

// ------ v4 ------

func TestGenerateUUIDv4_Format(t *testing.T) {
	s, err := GenerateUUIDv4(crypto_rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	checkUUIDStructure(t, s)
}

func TestGenerateUUIDv4_VersionNibble(t *testing.T) {
	// position 14 = version nibble (after dashes at 8, 13)
	for i := 0; i < 100; i++ {
		s, err := GenerateUUIDv4(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if s[14] != '4' {
			t.Fatalf("iter %d: v4 version nibble should be '4', got %q in %q", i, s[14], s)
		}
	}
}

func TestGenerateUUIDv4_VariantBits(t *testing.T) {
	// position 19 = variant nibble，应当是 8/9/a/b（高 2 bits = 10）
	for i := 0; i < 100; i++ {
		s, err := GenerateUUIDv4(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.ContainsRune("89ab", rune(s[19])) {
			t.Fatalf("iter %d: variant nibble should be 8/9/a/b, got %q in %q", i, s[19], s)
		}
	}
}

func TestGenerateUUIDv4_Unique(t *testing.T) {
	// 100 个 v4 UUID 不应有重复
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := GenerateUUIDv4(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if seen[s] {
			t.Fatalf("duplicate UUID: %q", s)
		}
		seen[s] = true
	}
}

// ------ v7 ------

func TestGenerateUUIDv7_Format(t *testing.T) {
	s, err := GenerateUUIDv7(crypto_rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	checkUUIDStructure(t, s)
}

func TestGenerateUUIDv7_VersionNibble(t *testing.T) {
	for i := 0; i < 100; i++ {
		s, err := GenerateUUIDv7(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if s[14] != '7' {
			t.Fatalf("iter %d: v7 version nibble should be '7', got %q in %q", i, s[14], s)
		}
	}
}

func TestGenerateUUIDv7_VariantBits(t *testing.T) {
	for i := 0; i < 100; i++ {
		s, err := GenerateUUIDv7(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.ContainsRune("89ab", rune(s[19])) {
			t.Fatalf("iter %d: variant nibble should be 8/9/a/b, got %q in %q", i, s[19], s)
		}
	}
}

func TestGenerateUUIDv7_TimestampMatchesNow(t *testing.T) {
	// 提取前 48 bits（前 12 hex char）作为 unix ms 时间戳，检查在当前时间附近
	before := time.Now().UnixMilli()
	s, err := GenerateUUIDv7(crypto_rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UnixMilli()

	cleaned := strings.ReplaceAll(s, "-", "")
	tsBytes, err := hex.DecodeString(cleaned[:12])
	if err != nil {
		t.Fatal(err)
	}

	var ms int64
	for _, b := range tsBytes {
		ms = (ms << 8) | int64(b)
	}

	// 给 ±100ms 的余量（系统时钟和 time.Now 的不确定性）
	if ms < before-100 || ms > after+100 {
		t.Errorf("UUID v7 timestamp %d outside [%d, %d]", ms, before-100, after+100)
	}
}

func TestGenerateUUIDv7_BatchOrdering(t *testing.T) {
	// 100 个 v7 UUID（每次 sleep 一点保证 ms 推进），前 8 hex chars 应当单调不减
	var uuids []string
	for i := 0; i < 50; i++ {
		s, err := GenerateUUIDv7(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		uuids = append(uuids, s)
		time.Sleep(1 * time.Millisecond)
	}
	// 前 8 hex 是高 32 bits 时间戳，应当大致单调递增
	for i := 1; i < len(uuids); i++ {
		if uuids[i][:8] < uuids[0][:8] {
			t.Errorf("UUID v7 batch not time-ordered: uuid[%d]=%s < uuid[0]=%s",
				i, uuids[i][:8], uuids[0][:8])
		}
	}
}

func TestGenerateUUIDv7_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s, err := GenerateUUIDv7(crypto_rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if seen[s] {
			t.Fatalf("duplicate UUID v7: %q", s)
		}
		seen[s] = true
	}
}
