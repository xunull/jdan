package fake

import (
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ---- 可复现性（核心）----

func TestSeedReproducible(t *testing.T) {
	a := New(42)
	b := New(42)
	for i := range 50 {
		if a.Name() != b.Name() || a.Int(0, 1000) != b.Int(0, 1000) || a.UUID() != b.UUID() {
			t.Fatalf("same seed should produce same sequence (iter %d)", i)
		}
	}
}

func TestDifferentSeedDiffers(t *testing.T) {
	a := New(1)
	b := New(2)
	// 不要求每个都不同，但整体序列应当不同
	same := true
	for range 20 {
		if a.Name() != b.Name() {
			same = false
		}
	}
	if same {
		t.Error("different seeds should diverge")
	}
}

func TestNewRandom(t *testing.T) {
	g, err := NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	if g.Name() == "" {
		t.Error("random generator should produce a name")
	}
}

// ---- 各类型格式 ----

func TestName(t *testing.T) {
	g := New(1)
	parts := strings.Fields(g.Name())
	if len(parts) != 2 {
		t.Errorf("name should be 'first last', got %q", g.Name())
	}
}

func TestEmail(t *testing.T) {
	g := New(1)
	e := g.Email()
	if !strings.Contains(e, "@") || !strings.Contains(e, ".") {
		t.Errorf("bad email %q", e)
	}
	// 局部部分应当小写
	local := strings.SplitN(e, "@", 2)[0]
	if local != strings.ToLower(local) {
		t.Errorf("email local part should be lowercase: %q", e)
	}
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUID(t *testing.T) {
	g := New(7)
	for range 100 {
		u := g.UUID()
		if !uuidRe.MatchString(u) {
			t.Fatalf("invalid v4 uuid: %q", u)
		}
	}
}

func TestWord(t *testing.T) {
	g := New(1)
	w := g.Word()
	if !slices.Contains(loremWords, w) {
		t.Errorf("word %q not in lorem list", w)
	}
}

func TestSentence(t *testing.T) {
	g := New(1)
	s := g.Sentence(4)
	if !strings.HasSuffix(s, ".") {
		t.Errorf("sentence should end with period: %q", s)
	}
	if s[0] < 'A' || s[0] > 'Z' {
		t.Errorf("sentence should start uppercase: %q", s)
	}
	words := strings.Fields(strings.TrimSuffix(s, "."))
	if len(words) != 4 {
		t.Errorf("expected 4 words, got %d: %q", len(words), s)
	}
}

func TestSentenceDefaultsToOne(t *testing.T) {
	g := New(1)
	s := g.Sentence(0)
	words := strings.Fields(strings.TrimSuffix(s, "."))
	if len(words) != 1 {
		t.Errorf("Sentence(0) should give 1 word, got %d", len(words))
	}
}

func TestInt(t *testing.T) {
	g := New(1)
	for range 1000 {
		n := g.Int(5, 10)
		if n < 5 || n > 10 {
			t.Fatalf("int %d out of range [5,10]", n)
		}
	}
}

func TestIntSwapsBounds(t *testing.T) {
	g := New(1)
	n := g.Int(10, 5) // 反序
	if n < 5 || n > 10 {
		t.Errorf("int %d should still be in [5,10]", n)
	}
}

func TestIntSingleValue(t *testing.T) {
	g := New(1)
	if n := g.Int(3, 3); n != 3 {
		t.Errorf("Int(3,3) should be 3, got %d", n)
	}
}

func TestDate(t *testing.T) {
	g := New(1)
	d := g.Date("")
	parsed, err := time.Parse("2006-01-02", d)
	if err != nil {
		t.Fatalf("date %q not parseable: %v", d, err)
	}
	if parsed.Year() < 2000 || parsed.Year() > 2024 {
		t.Errorf("date %q outside fixed window 2000-2024", d)
	}
}

func TestDateCustomFormat(t *testing.T) {
	g := New(1)
	d := g.Date("2006/01/02")
	if !strings.Contains(d, "/") {
		t.Errorf("custom format not applied: %q", d)
	}
}

func TestIP(t *testing.T) {
	g := New(1)
	for range 100 {
		ip := g.IP()
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			t.Fatalf("invalid IPv4: %q", ip)
		}
		// 必须落在文档保留段
		ok := false
		for _, b := range docIPBlocks {
			if strings.HasPrefix(ip, b+".") {
				ok = true
			}
		}
		if !ok {
			t.Errorf("ip %q not in a doc block", ip)
		}
	}
}

// ---- Value 分发 ----

func TestValue(t *testing.T) {
	g := New(1)
	for _, typ := range SupportedTypes {
		v, err := g.Value(typ, Options{Min: 0, Max: 10, Words: 3})
		if err != nil {
			t.Errorf("type %q errored: %v", typ, err)
		}
		if v == "" {
			t.Errorf("type %q produced empty value", typ)
		}
	}
}

func TestValueUnknownType(t *testing.T) {
	g := New(1)
	if _, err := g.Value("nope", Options{}); err == nil {
		t.Error("unknown type should error")
	}
}

func TestValueIntUsesRange(t *testing.T) {
	g := New(1)
	v, _ := g.Value("int", Options{Min: 100, Max: 100})
	if n, _ := strconv.Atoi(v); n != 100 {
		t.Errorf("int value should respect range, got %q", v)
	}
}

// ---- Person 复合记录 ----

func TestPerson(t *testing.T) {
	g := New(1)
	p := g.Person()
	if p.Name == "" || p.Email == "" || p.IP == "" {
		t.Errorf("person fields should be populated: %+v", p)
	}
	if p.Age < 18 || p.Age > 80 {
		t.Errorf("age %d out of [18,80]", p.Age)
	}
	// email 应与 name 一致：first.last
	first := strings.ToLower(strings.Fields(p.Name)[0])
	if !strings.HasPrefix(p.Email, first+".") {
		t.Errorf("email %q should derive from name %q", p.Email, p.Name)
	}
}

func TestPersonReproducible(t *testing.T) {
	a := New(99).Person()
	b := New(99).Person()
	if a != b {
		t.Errorf("same seed should give same person: %+v vs %+v", a, b)
	}
}

func TestSupportedTypesSorted(t *testing.T) {
	for i := 1; i < len(SupportedTypes); i++ {
		if SupportedTypes[i] < SupportedTypes[i-1] {
			t.Errorf("SupportedTypes not sorted at %d", i)
		}
	}
}
