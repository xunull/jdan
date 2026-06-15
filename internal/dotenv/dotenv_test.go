package dotenv

import (
	"reflect"
	"strings"
	"testing"
)

func parse(t *testing.T, s string) *File {
	t.Helper()
	f, err := Parse(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// ---- Parse ----

func TestParse_Basic(t *testing.T) {
	f := parse(t, "KEY=value\nFOO=bar\n")
	if len(f.Entries) != 2 {
		t.Fatalf("got %d entries", len(f.Entries))
	}
	if f.Entries[0].Key != "KEY" || f.Entries[0].Value != "value" {
		t.Errorf("entry 0 = %+v", f.Entries[0])
	}
	if f.Entries[0].Line != 1 || f.Entries[1].Line != 2 {
		t.Errorf("line numbers wrong")
	}
}

func TestParse_SkipsCommentsAndBlanks(t *testing.T) {
	f := parse(t, "# comment\n\nKEY=value\n\n# another\n")
	if len(f.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(f.Entries))
	}
	if f.Entries[0].Line != 3 {
		t.Errorf("line = %d, want 3", f.Entries[0].Line)
	}
}

func TestParse_ExportPrefix(t *testing.T) {
	f := parse(t, "export KEY=value\n")
	if !f.Entries[0].HadExport {
		t.Error("should detect export prefix")
	}
	if f.Entries[0].Key != "KEY" {
		t.Errorf("key = %q", f.Entries[0].Key)
	}
}

func TestParse_DoubleQuotes(t *testing.T) {
	f := parse(t, `KEY="value with spaces"`+"\n")
	if f.Entries[0].Value != "value with spaces" {
		t.Errorf("value = %q", f.Entries[0].Value)
	}
	if f.Entries[0].Quote != QuoteDouble {
		t.Errorf("quote = %v", f.Entries[0].Quote)
	}
}

func TestParse_SingleQuotes(t *testing.T) {
	f := parse(t, "KEY='literal'\n")
	if f.Entries[0].Value != "literal" || f.Entries[0].Quote != QuoteSingle {
		t.Errorf("entry = %+v", f.Entries[0])
	}
}

func TestParse_InlineComment(t *testing.T) {
	f := parse(t, "KEY=value # this is a comment\n")
	if f.Entries[0].Value != "value" {
		t.Errorf("inline comment not stripped: %q", f.Entries[0].Value)
	}
}

func TestParse_EmptyValue(t *testing.T) {
	f := parse(t, "KEY=\n")
	if f.Entries[0].Value != "" || !f.Entries[0].HasEquals {
		t.Errorf("entry = %+v", f.Entries[0])
	}
}

func TestParse_NoEquals(t *testing.T) {
	f := parse(t, "JUSTAKEY\n")
	if f.Entries[0].HasEquals {
		t.Error("should detect missing '='")
	}
}

func TestParse_CRLF(t *testing.T) {
	f := parse(t, "KEY=value\r\nFOO=bar\r\n")
	if !f.HasCRLF {
		t.Error("should detect CRLF")
	}
	if f.Entries[0].Value != "value" {
		t.Errorf("CRLF leaked into value: %q", f.Entries[0].Value)
	}
}

func TestParse_BOM(t *testing.T) {
	f := parse(t, "\ufeffKEY=value\n")
	if !f.HasBOM {
		t.Error("should detect BOM")
	}
	if f.Entries[0].Key != "KEY" {
		t.Errorf("BOM leaked into key: %q", f.Entries[0].Key)
	}
}

// ---- Lint ----

func TestLint_DuplicateKey(t *testing.T) {
	f := parse(t, "KEY=a\nKEY=b\n")
	issues := Lint(f)
	found := false
	for _, i := range issues {
		if i.Line == 2 && strings.Contains(i.Message, "duplicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("should flag duplicate, got %+v", issues)
	}
}

func TestLint_UnquotedSpaces(t *testing.T) {
	f := parse(t, "KEY=hello world\n")
	if !hasIssue(Lint(f), "unquoted value with spaces") {
		t.Error("should flag unquoted spaces")
	}
}

func TestLint_QuotedSpacesOK(t *testing.T) {
	f := parse(t, `KEY="hello world"`+"\n")
	if hasIssue(Lint(f), "unquoted value with spaces") {
		t.Error("quoted spaces should NOT be flagged")
	}
}

func TestLint_InvalidKeyName(t *testing.T) {
	f := parse(t, "2FOO=bad\n")
	issues := Lint(f)
	if !hasIssueSev(issues, "invalid key name", SevError) {
		t.Errorf("should flag invalid key as error, got %+v", issues)
	}
}

func TestLint_OrphanToken(t *testing.T) {
	f := parse(t, "JUSTAKEY\n")
	if !hasIssueSev(Lint(f), "no '='", SevError) {
		t.Error("orphan token should be error")
	}
}

func TestLint_TrailingWhitespace(t *testing.T) {
	f := parse(t, "KEY=value   \n")
	if !hasIssue(Lint(f), "trailing whitespace") {
		t.Error("should flag trailing whitespace")
	}
}

func TestLint_CleanFile(t *testing.T) {
	f := parse(t, "KEY=value\nFOO=bar\nNUM=42\n")
	if len(Lint(f)) != 0 {
		t.Errorf("clean file should have no issues, got %+v", Lint(f))
	}
}

func TestLint_BOMAndCRLF(t *testing.T) {
	f := parse(t, "\ufeffKEY=value\r\n")
	issues := Lint(f)
	if !hasIssue(issues, "BOM") {
		t.Error("should flag BOM")
	}
	if !hasIssue(issues, "CRLF") {
		t.Error("should flag CRLF")
	}
}

func TestCountBySeverity(t *testing.T) {
	issues := []Issue{
		{Severity: SevError}, {Severity: SevWarning}, {Severity: SevWarning},
	}
	e, w := CountBySeverity(issues)
	if e != 1 || w != 2 {
		t.Errorf("got errors=%d warnings=%d", e, w)
	}
}

// ---- Diff ----

func TestDiff_KeySets(t *testing.T) {
	a := parse(t, "SHARED=1\nONLY_A=2\n")
	b := parse(t, "SHARED=1\nONLY_B=3\n")
	res := Diff(a, b, false)
	if !reflect.DeepEqual(res.OnlyInA, []string{"ONLY_A"}) {
		t.Errorf("OnlyInA = %v", res.OnlyInA)
	}
	if !reflect.DeepEqual(res.OnlyInB, []string{"ONLY_B"}) {
		t.Errorf("OnlyInB = %v", res.OnlyInB)
	}
	if !reflect.DeepEqual(res.Common, []string{"SHARED"}) {
		t.Errorf("Common = %v", res.Common)
	}
}

func TestDiff_NoValuesWithoutFlag(t *testing.T) {
	a := parse(t, "KEY=a\n")
	b := parse(t, "KEY=b\n")
	res := Diff(a, b, false)
	if len(res.ValueDiff) != 0 {
		t.Error("should not compare values without withValues")
	}
	if res.HasDifferences() {
		t.Error("same keys = no differences when not comparing values")
	}
}

func TestDiff_WithValues(t *testing.T) {
	a := parse(t, "KEY=a\n")
	b := parse(t, "KEY=b\n")
	res := Diff(a, b, true)
	if len(res.ValueDiff) != 1 || res.ValueDiff[0].Key != "KEY" {
		t.Errorf("ValueDiff = %+v", res.ValueDiff)
	}
	if !res.HasDifferences() {
		t.Error("differing values = differences")
	}
}

func TestDiff_DuplicateKeyTakesLast(t *testing.T) {
	a := parse(t, "KEY=first\nKEY=last\n")
	b := parse(t, "KEY=last\n")
	res := Diff(a, b, true)
	if len(res.ValueDiff) != 0 {
		t.Errorf("dup key should take last value (shell semantics): %+v", res.ValueDiff)
	}
}

// ---- Redact ----

func TestRedactValue_Long(t *testing.T) {
	got := RedactValue("sk-abc123def456", RedactOpts{})
	// 长值保留首尾各 2
	if !strings.HasPrefix(got, "sk") || !strings.HasSuffix(got, "56") {
		t.Errorf("got %q", got)
	}
	if strings.Contains(got, "abc123def") {
		t.Errorf("middle not masked: %q", got)
	}
}

func TestRedactValue_Full(t *testing.T) {
	if got := RedactValue("anything", RedactOpts{Full: true}); got != "****" {
		t.Errorf("got %q", got)
	}
}

func TestRedactValue_KeepShort(t *testing.T) {
	if got := RedactValue("true", RedactOpts{KeepShort: true}); got != "true" {
		t.Errorf("boolish should be kept: %q", got)
	}
	if got := RedactValue("8080", RedactOpts{KeepShort: true}); got != "8080" {
		t.Errorf("short should be kept: %q", got)
	}
}

func TestRedactValue_Empty(t *testing.T) {
	if got := RedactValue("", RedactOpts{}); got != "" {
		t.Errorf("empty should stay empty: %q", got)
	}
}

func TestRedactLine_PreservesExport(t *testing.T) {
	f := parse(t, "export SECRET=verylongsecret\n")
	line := RedactLine(f.Entries[0], RedactOpts{})
	if !strings.HasPrefix(line, "export SECRET=") {
		t.Errorf("export prefix lost: %q", line)
	}
	if strings.Contains(line, "verylongsecret") {
		t.Errorf("value not redacted: %q", line)
	}
}

// ---- Get ----

func TestGet_Found(t *testing.T) {
	f := parse(t, `KEY="quoted value"`+"\n")
	v, err := Get(f, "KEY")
	if err != nil {
		t.Fatal(err)
	}
	if v != "quoted value" {
		t.Errorf("got %q", v)
	}
}

func TestGet_NotFound(t *testing.T) {
	f := parse(t, "KEY=value\n")
	if _, err := Get(f, "NOPE"); err == nil {
		t.Error("missing key should error")
	}
}

func TestGet_DuplicateTakesLast(t *testing.T) {
	f := parse(t, "KEY=first\nKEY=last\n")
	v, _ := Get(f, "KEY")
	if v != "last" {
		t.Errorf("got %q, want last", v)
	}
}

// ---- helpers ----

func hasIssue(issues []Issue, substr string) bool {
	for _, i := range issues {
		if strings.Contains(i.Message, substr) {
			return true
		}
	}
	return false
}

func hasIssueSev(issues []Issue, substr string, sev Severity) bool {
	for _, i := range issues {
		if i.Severity == sev && strings.Contains(i.Message, substr) {
			return true
		}
	}
	return false
}
