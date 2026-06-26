package morsex

import (
	"strings"
	"testing"
)

// ---- Encode ----

func TestEncode_SOS(t *testing.T) {
	got, skipped := Encode("SOS")
	if got != "... --- ..." || skipped != 0 {
		t.Errorf("got %q skipped %d", got, skipped)
	}
}

func TestEncode_CaseInsensitive(t *testing.T) {
	lo, _ := Encode("sos")
	up, _ := Encode("SOS")
	if lo != up {
		t.Errorf("lowercase %q != uppercase %q", lo, up)
	}
}

func TestEncode_WordSeparator(t *testing.T) {
	got, _ := Encode("HELLO WORLD")
	if !strings.Contains(got, " / ") {
		t.Errorf("words should be separated by ' / ': %q", got)
	}
}

func TestEncode_DigitsAndPunct(t *testing.T) {
	got, skipped := Encode("1?")
	if got != ".---- ..--.." || skipped != 0 {
		t.Errorf("got %q skipped %d", got, skipped)
	}
}

func TestEncode_SkipsUnknown(t *testing.T) {
	got, skipped := Encode("hi你好")
	if got != ".... .." || skipped != 2 {
		t.Errorf("got %q skipped %d, want '.... ..' skipped 2", got, skipped)
	}
}

func TestEncode_Empty(t *testing.T) {
	if got, _ := Encode(""); got != "" {
		t.Errorf("empty → %q", got)
	}
}

// ---- Decode ----

func TestDecode_SOS(t *testing.T) {
	got, unknown := Decode("... --- ...")
	if got != "SOS" || unknown != 0 {
		t.Errorf("got %q unknown %d", got, unknown)
	}
}

func TestDecode_WordSeparator(t *testing.T) {
	got, _ := Decode(".... . .-.. .-.. --- / .-- --- .-. .-.. -..")
	if got != "HELLO WORLD" {
		t.Errorf("got %q", got)
	}
}

func TestDecode_ExtraSpacesTolerated(t *testing.T) {
	got, _ := Decode("...    ---   ...")
	if got != "SOS" {
		t.Errorf("extra spaces should collapse: %q", got)
	}
}

func TestDecode_UnknownToken(t *testing.T) {
	got, unknown := Decode("... ........ ...")
	if unknown != 1 || !strings.Contains(got, "#") {
		t.Errorf("unknown token → '#' + count: got %q unknown %d", got, unknown)
	}
}

// ---- round-trip ----

func TestRoundTrip(t *testing.T) {
	for _, in := range []string{"SOS", "HELLO WORLD", "ABC 123", "WHY NOT?", "A B C"} {
		code, skipped := Encode(in)
		if skipped != 0 {
			t.Fatalf("%q unexpectedly skipped %d", in, skipped)
		}
		back, unknown := Decode(code)
		if unknown != 0 {
			t.Fatalf("%q decode unknown %d", in, unknown)
		}
		if back != strings.ToUpper(in) {
			t.Errorf("round-trip %q → %q → %q", in, code, back)
		}
	}
}

// ---- LooksLikeMorse ----

func TestLooksLikeMorse(t *testing.T) {
	cases := map[string]bool{
		"... --- ...": true,
		".-/.-":       true,
		"  . - / .  ": true,
		"SOS":         false,
		"hello":       false,
		"":            false,
		"  ":          false,
		".-x":         false,
	}
	for in, want := range cases {
		if got := LooksLikeMorse(in); got != want {
			t.Errorf("LooksLikeMorse(%q) = %v, want %v", in, got, want)
		}
	}
}
