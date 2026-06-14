package hash

import (
	"strings"
	"testing"
)

// 经典 test vector：MD5 / SHA1 / SHA256 / SHA512 of empty string
// 这些跟 RFC / NIST / openssl 输出对齐。
const (
	emptyMD5    = "d41d8cd98f00b204e9800998ecf8427e"
	emptySHA1   = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	emptySHA512 = "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"
)

// "abc" hashes (from FIPS / Wikipedia)
const (
	abcMD5    = "900150983cd24fb0d6963f7d28e17f72"
	abcSHA1   = "a9993e364706816aba3e25717850c26c9cd0d89d"
	abcSHA256 = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	abcSHA512 = "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"
)

func TestHashReader_EmptyString_AllAlgos(t *testing.T) {
	r := strings.NewReader("")
	got, err := HashReader(r, AllAlgorithms())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		algo Algorithm
		want string
	}{
		{AlgoMD5, emptyMD5},
		{AlgoSHA1, emptySHA1},
		{AlgoSHA256, emptySHA256},
		{AlgoSHA512, emptySHA512},
	} {
		if g := got.Sum(tc.algo); g != tc.want {
			t.Errorf("%s of empty string = %s, want %s", tc.algo, g, tc.want)
		}
	}
}

func TestHashReader_ABC_KnownValues(t *testing.T) {
	for _, algo := range AllAlgorithms() {
		r := strings.NewReader("abc")
		got, err := HashReader(r, []Algorithm{algo})
		if err != nil {
			t.Fatal(err)
		}
		var want string
		switch algo {
		case AlgoMD5:
			want = abcMD5
		case AlgoSHA1:
			want = abcSHA1
		case AlgoSHA256:
			want = abcSHA256
		case AlgoSHA512:
			want = abcSHA512
		}
		if got.Sum(algo) != want {
			t.Errorf("%s of \"abc\" = %s, want %s", algo, got.Sum(algo), want)
		}
	}
}

func TestHashReader_MultiAlgoSinglePass(t *testing.T) {
	// 同时跑 md5 + sha256，结果应该跟单独跑一致（验证 MultiWriter 路径）
	r := strings.NewReader("abc")
	got, err := HashReader(r, []Algorithm{AlgoMD5, AlgoSHA256})
	if err != nil {
		t.Fatal(err)
	}
	if got.Sum(AlgoMD5) != abcMD5 {
		t.Errorf("multi-algo MD5 lost: %s", got.Sum(AlgoMD5))
	}
	if got.Sum(AlgoSHA256) != abcSHA256 {
		t.Errorf("multi-algo SHA256 lost: %s", got.Sum(AlgoSHA256))
	}
}

func TestHashReader_NoAlgos_Errors(t *testing.T) {
	if _, err := HashReader(strings.NewReader("x"), nil); err == nil {
		t.Error("no algos should error")
	}
}

func TestParseAlgorithms(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []Algorithm
	}{
		{"sha256", []Algorithm{AlgoSHA256}},
		{"md5,sha256", []Algorithm{AlgoMD5, AlgoSHA256}},
		{"MD5,SHA256", []Algorithm{AlgoMD5, AlgoSHA256}},                                // 大小写不敏感
		{"sha256,sha256,md5", []Algorithm{AlgoSHA256, AlgoMD5}},                          // dedup
		{"  md5  ,  sha1  ", []Algorithm{AlgoMD5, AlgoSHA1}},                            // trim
	} {
		got, err := ParseAlgorithms(tc.in)
		if err != nil {
			t.Errorf("ParseAlgorithms(%q) error: %v", tc.in, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("ParseAlgorithms(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ParseAlgorithms(%q)[%d] = %s, want %s", tc.in, i, got[i], tc.want[i])
			}
		}
	}
	for _, bad := range []string{"", "  ", "sha3", "md5,bogus"} {
		if _, err := ParseAlgorithms(bad); err == nil {
			t.Errorf("ParseAlgorithms(%q) should error", bad)
		}
	}
}

func TestParseChecksumLine_Variants(t *testing.T) {
	for _, tc := range []struct {
		in         string
		wantPath   string
		wantHash   string
		wantNil    bool
		wantErr    bool
	}{
		// 标准 shasum: 两空格分隔
		{"abc123  file.txt", "file.txt", "abc123", false, false},
		// binary mode: ` *` 分隔
		{"abc123 *file.bin", "file.bin", "abc123", false, false},
		// 大写 hash 应当被 lowercase
		{"ABC123  upper.txt", "upper.txt", "abc123", false, false},
		// 空行
		{"", "", "", true, false},
		// 注释
		{"# comment", "", "", true, false},
		// 完全 malformed
		{"no spaces here", "", "", false, true},
	} {
		got, err := ParseChecksumLine(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q should error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q unexpected error: %v", tc.in, err)
			continue
		}
		if tc.wantNil {
			if got != nil {
				t.Errorf("%q should give nil entry, got %+v", tc.in, got)
			}
			continue
		}
		if got.Path != tc.wantPath || got.Expected != tc.wantHash {
			t.Errorf("ParseChecksumLine(%q) = {%q, %q}, want {%q, %q}",
				tc.in, got.Path, got.Expected, tc.wantPath, tc.wantHash)
		}
	}
}

func TestParseChecksumFile_MultilineFiltersEmpties(t *testing.T) {
	input := `# checksums
abc123  a.txt

def456  b.txt
# another comment
99aa  c.bin`
	entries, err := ParseChecksumFile(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
}

func TestParseChecksumFile_MalformedLineErrors(t *testing.T) {
	input := "abc123  good.txt\nbad-line-no-space"
	_, err := ParseChecksumFile(strings.NewReader(input))
	if err == nil {
		t.Error("malformed line should error")
	}
}

func TestAlgoFromHexLength(t *testing.T) {
	for _, tc := range []struct {
		hex  string
		want Algorithm
	}{
		{strings.Repeat("a", 32), AlgoMD5},
		{strings.Repeat("a", 40), AlgoSHA1},
		{strings.Repeat("a", 64), AlgoSHA256},
		{strings.Repeat("a", 128), AlgoSHA512},
	} {
		got, err := AlgoFromHexLength(tc.hex)
		if err != nil {
			t.Errorf("len %d should be %s, got error: %v", len(tc.hex), tc.want, err)
			continue
		}
		if got != tc.want {
			t.Errorf("len %d = %s, want %s", len(tc.hex), got, tc.want)
		}
	}
	if _, err := AlgoFromHexLength("xx"); err == nil {
		t.Error("ambiguous length should error")
	}
}

func TestResult_SortedAlgos(t *testing.T) {
	r := &Result{Sums: map[Algorithm]string{
		AlgoSHA512: "x", AlgoMD5: "y", AlgoSHA256: "z",
	}}
	algos := r.SortedAlgos()
	want := []Algorithm{AlgoMD5, AlgoSHA256, AlgoSHA512}
	for i, a := range algos {
		if i >= len(want) || a != want[i] {
			t.Errorf("SortedAlgos[%d] = %s, want %s", i, a, want[i])
		}
	}
}

func TestResult_Sum_Nil(t *testing.T) {
	if got := (*Result)(nil).Sum(AlgoSHA256); got != "" {
		t.Errorf("nil Result Sum should give empty, got %q", got)
	}
}
