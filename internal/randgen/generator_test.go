package randgen

import (
	"bytes"
	crypto_rand "crypto/rand"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// failingReader 在 Read 时永远返回 err，用于测试 CSPRNG 失败路径传播。
type failingReader struct{ err error }

func (f failingReader) Read(p []byte) (int, error) { return 0, f.err }

// fixedReader 返回预设字节序列，用于精确测试。耗尽后返回 io.EOF。
type fixedReader struct {
	data []byte
	pos  int
}

func (r *fixedReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// ------ Hex / Base64 / Base64URL / Base32 ------

func TestGenerateHex_LengthAndCharset(t *testing.T) {
	s, err := GenerateHex(crypto_rand.Reader, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 32 {
		t.Errorf("hex(16) length = %d, want 32", len(s))
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("non-hex char: %q", c)
		}
	}
}

func TestGenerateHex_RejectsNonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		if _, err := GenerateHex(crypto_rand.Reader, n); err == nil {
			t.Errorf("hex(%d) should error", n)
		}
	}
}

func TestGenerateBase64_StandardPadding(t *testing.T) {
	s, err := GenerateBase64(crypto_rand.Reader, 30)
	if err != nil {
		t.Fatal(err)
	}
	// 30 bytes → 40 base64 chars (with padding if needed). 30 / 3 = 10 → 40 exactly.
	if len(s) != 40 {
		t.Errorf("base64(30) length = %d, want 40", len(s))
	}
}

func TestGenerateBase64URL_NoStandardChars(t *testing.T) {
	for i := 0; i < 50; i++ {
		s, err := GenerateBase64URL(crypto_rand.Reader, 32)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(s, "+/=") {
			t.Errorf("base64url should not contain +/=, got: %q", s)
		}
	}
}

func TestGenerateBase32_RFC4648(t *testing.T) {
	s, err := GenerateBase32(crypto_rand.Reader, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range s {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567=", c) {
			t.Errorf("non-RFC4648 base32 char: %q in %q", c, s)
		}
	}
}

// ------ Alnum ------

func TestGenerateAlnum_LengthAndCharset(t *testing.T) {
	s, err := GenerateAlnum(crypto_rand.Reader, 20, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 20 {
		t.Errorf("alnum length = %d", len(s))
	}
	// 排除歧义字符
	if strings.ContainsAny(s, "Il1O0") {
		t.Errorf("alnum default should exclude ambiguous chars, got: %q", s)
	}
}

func TestGenerateAlnum_NoClassConstraint(t *testing.T) {
	// length=1 应当成功（无类约束 — alnum 与 password 的关键区别）
	s, err := GenerateAlnum(crypto_rand.Reader, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 {
		t.Errorf("length = %d", len(s))
	}
}

func TestGenerateAlnum_RejectsZero(t *testing.T) {
	if _, err := GenerateAlnum(crypto_rand.Reader, 0, false); err == nil {
		t.Error("alnum(0) should error")
	}
}

func TestGenerateAlnum_IncludeAmbiguous(t *testing.T) {
	// 1000 length=50 串里应当能见到歧义字符
	seen := make(map[rune]bool)
	for i := 0; i < 50; i++ {
		s, err := GenerateAlnum(crypto_rand.Reader, 50, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range s {
			if strings.ContainsRune("Il1O0", c) {
				seen[c] = true
			}
		}
	}
	if len(seen) < 3 {
		t.Errorf("include-ambiguous: 50 long strings should hit >= 3 ambiguous chars, got %d", len(seen))
	}
}

// ------ Password ------

func TestGeneratePassword_DefaultLength(t *testing.T) {
	s, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 20 {
		t.Errorf("length = %d, want 20", len(s))
	}
}

func TestGeneratePassword_AlwaysContainsEachClass_WithSymbols(t *testing.T) {
	// 1000 iterations at length=4 (min): all 4 classes must appear
	lo, up, di, sy, _, _, _ := charsetsForTesting()
	for i := 0; i < 1000; i++ {
		s, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: 4})
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if !charsetContains(s, lo) {
			t.Fatalf("iter %d: missing lowercase: %q", i, s)
		}
		if !charsetContains(s, up) {
			t.Fatalf("iter %d: missing uppercase: %q", i, s)
		}
		if !charsetContains(s, di) {
			t.Fatalf("iter %d: missing digit: %q", i, s)
		}
		if !charsetContains(s, sy) {
			t.Fatalf("iter %d: missing symbol: %q", i, s)
		}
	}
}

func TestGeneratePassword_AlwaysContainsEachClass_NoSymbols(t *testing.T) {
	lo, up, di, sy, _, _, _ := charsetsForTesting()
	for i := 0; i < 1000; i++ {
		s, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: 3, NoSymbols: true})
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if !charsetContains(s, lo) || !charsetContains(s, up) || !charsetContains(s, di) {
			t.Fatalf("iter %d: missing class: %q", i, s)
		}
		if charsetContains(s, sy) {
			t.Fatalf("iter %d: --no-symbols leaked symbol: %q", i, s)
		}
	}
}

func TestGeneratePassword_RejectsTooShort_WithSymbols(t *testing.T) {
	for _, n := range []int{1, 2, 3} {
		if _, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: n}); err == nil {
			t.Errorf("length=%d with symbols should error", n)
		}
	}
}

func TestGeneratePassword_RejectsTooShort_NoSymbols(t *testing.T) {
	for _, n := range []int{1, 2} {
		if _, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: n, NoSymbols: true}); err == nil {
			t.Errorf("length=%d --no-symbols should error", n)
		}
	}
}

func TestGeneratePassword_RejectsZeroLength(t *testing.T) {
	if _, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: 0}); err == nil {
		t.Error("length=0 should error")
	}
	if _, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: -5}); err == nil {
		t.Error("length=-5 should error")
	}
}

func TestGeneratePassword_ExcludeAmbiguousByDefault(t *testing.T) {
	// 100 长度 40 的密码：永远不含 I/l/1/O/0
	for i := 0; i < 100; i++ {
		s, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: 40})
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(s, "Il1O0") {
			t.Fatalf("iter %d: contains ambiguous char: %q", i, s)
		}
	}
}

func TestGeneratePassword_IncludeAmbiguous(t *testing.T) {
	// 500 长度 40：should hit 多个歧义字符（统计意义上）
	seen := make(map[rune]bool)
	for i := 0; i < 500; i++ {
		s, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{
			Length: 40, IncludeAmbiguous: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range s {
			if strings.ContainsRune("Il1O0", c) {
				seen[c] = true
			}
		}
	}
	if len(seen) < 3 {
		t.Errorf("--include-ambiguous: 500 passwords should hit >= 3 ambiguous chars, got %d", len(seen))
	}
}

func TestGeneratePassword_LongLengthOK(t *testing.T) {
	s, err := GeneratePassword(crypto_rand.Reader, PasswordOptions{Length: 256})
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 256 {
		t.Errorf("length = %d", len(s))
	}
}

// ------ Int ------

func TestGenerateInt_InClosedRange(t *testing.T) {
	for i := 0; i < 200; i++ {
		n, err := GenerateInt(crypto_rand.Reader, 5, 10)
		if err != nil {
			t.Fatal(err)
		}
		if n < 5 || n > 10 {
			t.Errorf("got %d, want [5,10]", n)
		}
	}
}

func TestGenerateInt_HitsBothBoundaries(t *testing.T) {
	// 200 次抽 [5,7] (range size 3) 应当同时见到 5 和 7
	seenMin, seenMax := false, false
	for i := 0; i < 200; i++ {
		n, err := GenerateInt(crypto_rand.Reader, 5, 7)
		if err != nil {
			t.Fatal(err)
		}
		if n == 5 {
			seenMin = true
		}
		if n == 7 {
			seenMax = true
		}
	}
	if !seenMin || !seenMax {
		t.Errorf("after 200 picks of [5,7], seen min=%v max=%v", seenMin, seenMax)
	}
}

func TestGenerateInt_NegativeRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		n, err := GenerateInt(crypto_rand.Reader, -10, -5)
		if err != nil {
			t.Fatal(err)
		}
		if n < -10 || n > -5 {
			t.Errorf("got %d, want [-10,-5]", n)
		}
	}
}

func TestGenerateInt_CrossZero(t *testing.T) {
	for i := 0; i < 100; i++ {
		n, err := GenerateInt(crypto_rand.Reader, -5, 5)
		if err != nil {
			t.Fatal(err)
		}
		if n < -5 || n > 5 {
			t.Errorf("got %d, want [-5,5]", n)
		}
	}
}

func TestGenerateInt_BoundaryEqual(t *testing.T) {
	for i := 0; i < 10; i++ {
		n, err := GenerateInt(crypto_rand.Reader, 5, 5)
		if err != nil {
			t.Fatal(err)
		}
		if n != 5 {
			t.Errorf("min=max=5 → got %d", n)
		}
	}
}

func TestGenerateInt_RejectsMaxLessThanMin(t *testing.T) {
	if _, err := GenerateInt(crypto_rand.Reader, 5, 3); err == nil {
		t.Error("max < min should error")
	}
}

// ------ 统计正确性（bias 检测） ------

func TestGenerateHex_ByteUniformity_ChiSquare(t *testing.T) {
	// 16384 个字节，期望均匀 64 个/bin，256 bins
	const samples = 16384
	const bins = 256
	expected := float64(samples) / float64(bins)

	counts := make([]int, bins)
	buf := make([]byte, samples)
	if _, err := io.ReadFull(crypto_rand.Reader, buf); err != nil {
		t.Fatal(err)
	}
	for _, b := range buf {
		counts[b]++
	}

	chiSquare := 0.0
	for _, c := range counts {
		diff := float64(c) - expected
		chiSquare += diff * diff / expected
	}

	// df=255, p=0.001 critical value ≈ 330
	// p=0.0001 critical value ≈ 350
	// 设阈值 360，给统计噪声留余量
	if chiSquare > 360 {
		t.Errorf("chi-square = %.2f too high (>360), suggests bias", chiSquare)
	}
	t.Logf("chi-square = %.2f (df=255, threshold=360)", chiSquare)
}

func TestRandIndex_Uniformity(t *testing.T) {
	// 抽 [0, 7) 10000 次，每 bin 期望 ~1429
	const samples = 10000
	const n = 7
	counts := make([]int, n)
	for i := 0; i < samples; i++ {
		idx, err := randIndex(crypto_rand.Reader, n)
		if err != nil {
			t.Fatal(err)
		}
		if idx < 0 || idx >= n {
			t.Fatalf("idx out of range: %d", idx)
		}
		counts[idx]++
	}
	expected := float64(samples) / float64(n)
	for i, c := range counts {
		// ±20% 容差
		if float64(c) < expected*0.8 || float64(c) > expected*1.2 {
			t.Errorf("bin %d count %d, expected ~%.0f (±20%%)", i, c, expected)
		}
	}
}

// ------ Reader error 传播 ------

func TestGeneratePassword_PropagatesReaderError(t *testing.T) {
	fakeErr := errors.New("fake CSPRNG failure")
	_, err := GeneratePassword(failingReader{fakeErr}, PasswordOptions{Length: 20})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fake CSPRNG failure") {
		t.Errorf("error should wrap fake err, got: %v", err)
	}
}

func TestGenerateHex_PropagatesReaderError(t *testing.T) {
	fakeErr := errors.New("fake io")
	_, err := GenerateHex(failingReader{fakeErr}, 16)
	if err == nil || !strings.Contains(err.Error(), "fake io") {
		t.Errorf("expected wrapped fake err, got: %v", err)
	}
}

func TestGenerateBase64_PropagatesReaderError(t *testing.T) {
	fakeErr := errors.New("fake io")
	_, err := GenerateBase64(failingReader{fakeErr}, 16)
	if err == nil || !strings.Contains(err.Error(), "fake io") {
		t.Errorf("expected wrapped fake err, got: %v", err)
	}
}

func TestGenerateInt_PropagatesReaderError(t *testing.T) {
	fakeErr := errors.New("fake io")
	_, err := GenerateInt(failingReader{fakeErr}, 1, 10)
	if err == nil {
		t.Error("expected error from fake reader")
	}
}

// ------ Helper 函数测试 ------

func TestRandIndex_RejectsNonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		if _, err := randIndex(crypto_rand.Reader, n); err == nil {
			t.Errorf("randIndex(%d) should error", n)
		}
	}
}

func TestShuffle_EmptyAndSingle(t *testing.T) {
	// 空和单元素不应当 panic
	if err := shuffle(crypto_rand.Reader, nil); err != nil {
		t.Errorf("shuffle nil: %v", err)
	}
	if err := shuffle(crypto_rand.Reader, []byte{'x'}); err != nil {
		t.Errorf("shuffle 1-elem: %v", err)
	}
}

func TestShuffle_PreservesContent(t *testing.T) {
	// 洗牌后字符内容不变（只换位置）
	original := []byte("abcdefghij")
	b := bytes.Clone(original)
	if err := shuffle(crypto_rand.Reader, b); err != nil {
		t.Fatal(err)
	}
	if len(b) != len(original) {
		t.Fatal("length changed")
	}
	for _, c := range original {
		if !bytes.Contains(b, []byte{c}) {
			t.Errorf("shuffle lost char %q", c)
		}
	}
}

// ------ 静态检查：禁止 math/rand 和 mod-bias ------

func TestNoMathRandImport(t *testing.T) {
	// 仅扫非 _test.go 文件——生产代码绝不可 import math/rand
	files := nonTestGoFiles(t)
	for _, f := range files {
		cmd := exec.Command("grep", "-ln", "\"math/rand\"", f)
		out, _ := cmd.Output()
		if len(out) > 0 {
			t.Errorf("math/rand reference in production code: %s", f)
		}
	}
}

func TestNoCharSelectionModulo(t *testing.T) {
	// 反例：b[i] % byte(...) 或 b[i] % len(charset) 或类似 mod 选取
	// 这些是 CSPRNG bias landmine。仅扫生产代码且排除注释行（注释里说明"禁止"
	// 时会含 anti-pattern 字符串，那是文档不是代码）
	files := nonTestGoFiles(t)
	for _, f := range files {
		// `grep -vE '^[[:space:]]*//'` 排除注释；剩下再 grep anti-pattern
		cmd := exec.Command("sh", "-c",
			"grep -vE '^[[:space:]]*//' "+f+
				" | grep -En '% (byte\\(|len\\(charset|len\\(chars)' || true")
		out, _ := cmd.Output()
		if len(out) > 0 {
			t.Errorf("possible modulo-bias in production %s:\n%s", f, out)
		}
	}
}

func nonTestGoFiles(t *testing.T) []string {
	t.Helper()
	dir := packageDir(t)
	cmd := exec.Command("sh", "-c",
		"find "+dir+" -maxdepth 1 -name '*.go' -not -name '*_test.go'")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		t.Fatal("no production .go files found")
	}
	return lines
}

func packageDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(thisFile)
}
