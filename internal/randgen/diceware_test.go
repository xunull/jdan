package randgen

import (
	crypto_rand "crypto/rand"
	"errors"
	"strings"
	"testing"
)

func TestDicewareCount_Is7776(t *testing.T) {
	if DicewareCount() != 7776 {
		t.Errorf("expected 7776 EFF words, got %d", DicewareCount())
	}
}

func TestGenerateWords_Count(t *testing.T) {
	s, err := GenerateWords(crypto_rand.Reader, 6, "-")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(s, "-")
	if len(parts) != 6 {
		t.Errorf("expected 6 parts, got %d (s=%q)", len(parts), s)
	}
	for i, p := range parts {
		if p == "" {
			t.Errorf("part %d empty", i)
		}
	}
}

func TestGenerateWords_DifferentSeparators(t *testing.T) {
	for _, sep := range []string{"-", "_", ".", " ", "/", "::"} {
		s, err := GenerateWords(crypto_rand.Reader, 4, sep)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s, sep) {
			t.Errorf("sep %q not in output: %q", sep, s)
		}
	}
}

func TestGenerateWords_EmptySep(t *testing.T) {
	// 空 sep + count 3 = 三个词拼起来不可分割串。是用户的选择，不是错误
	s, err := GenerateWords(crypto_rand.Reader, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, " ") || strings.Contains(s, "-") {
		t.Errorf("empty sep should produce unbroken string, got %q", s)
	}
	// 不应是空串
	if len(s) == 0 {
		t.Error("got empty result")
	}
}

func TestGenerateWords_SingleWord(t *testing.T) {
	s, err := GenerateWords(crypto_rand.Reader, 1, "-")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s, "-") {
		t.Errorf("single word should have no sep, got %q", s)
	}
	if len(s) == 0 {
		t.Error("got empty")
	}
}

func TestGenerateWords_RejectsNonPositiveCount(t *testing.T) {
	for _, c := range []int{0, -1, -100} {
		if _, err := GenerateWords(crypto_rand.Reader, c, "-"); err == nil {
			t.Errorf("count=%d should error", c)
		}
	}
}

func TestGenerateWords_UsesFullList(t *testing.T) {
	// 抽 500 个单词，应当见到 >= 100 个不同的词（EFF 有 7776 词）
	seen := make(map[string]bool)
	for i := 0; i < 500; i++ {
		s, err := GenerateWords(crypto_rand.Reader, 1, "")
		if err != nil {
			t.Fatal(err)
		}
		seen[s] = true
	}
	if len(seen) < 100 {
		t.Errorf("only %d unique words in 500 picks; suspect non-uniform distribution", len(seen))
	}
}

func TestGenerateWords_WordsAreLowercase(t *testing.T) {
	// EFF list 应当全是小写
	for i := 0; i < 100; i++ {
		s, err := GenerateWords(crypto_rand.Reader, 1, "")
		if err != nil {
			t.Fatal(err)
		}
		if s != strings.ToLower(s) {
			t.Errorf("EFF word not lowercase: %q", s)
		}
	}
}

func TestGenerateWords_PropagatesReaderError(t *testing.T) {
	fakeErr := errors.New("fake io")
	_, err := GenerateWords(failingReader{fakeErr}, 6, "-")
	if err == nil {
		t.Error("expected error from failing reader")
	}
}
