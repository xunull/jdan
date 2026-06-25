package toc

import (
	"strings"
	"testing"
)

// ---- Slug ----

func TestSlug(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Hello World", "hello-world"},
		{"`jdan http timing`", "jdan-http-timing"},
		{"安装", "安装"},
		{"方式 1：下载（推荐）", "方式-1下载推荐"},
		{"C#", "c"},
		{"foo_bar-baz", "foo_bar-baz"},
		{"Trailing!!!", "trailing"},
		{"UPPER Case", "upper-case"},
	}
	for _, c := range cases {
		if got := Slug(c.in); got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---- ParseHeadings ----

func TestParseHeadings_Basic(t *testing.T) {
	md := "# Title\n\n## Setup\n\n### Detail\n"
	hs := ParseHeadings(md)
	if len(hs) != 3 {
		t.Fatalf("got %d headings, want 3", len(hs))
	}
	if hs[0].Level != 1 || hs[0].Text != "Title" || hs[0].Anchor != "title" {
		t.Errorf("bad h1: %+v", hs[0])
	}
	if hs[2].Level != 3 || hs[2].Anchor != "detail" {
		t.Errorf("bad h3: %+v", hs[2])
	}
}

func TestParseHeadings_SkipsCodeFence(t *testing.T) {
	md := "# Real\n\n```\n# fake in code\n```\n\n## Also Real\n"
	hs := ParseHeadings(md)
	if len(hs) != 2 {
		t.Fatalf("got %d headings, want 2 (code fence # must be skipped): %+v", len(hs), hs)
	}
	if hs[1].Text != "Also Real" {
		t.Errorf("second heading = %q, want 'Also Real'", hs[1].Text)
	}
}

func TestParseHeadings_TildeFence(t *testing.T) {
	md := "# Real\n~~~\n# fake\n~~~\n## Two\n"
	hs := ParseHeadings(md)
	if len(hs) != 2 {
		t.Errorf("tilde fence not handled: %+v", hs)
	}
}

func TestParseHeadings_NoSpaceNotHeading(t *testing.T) {
	md := "#nospace\n## real\n"
	hs := ParseHeadings(md)
	if len(hs) != 1 || hs[0].Text != "real" {
		t.Errorf("'#nospace' should not be a heading: %+v", hs)
	}
}

func TestParseHeadings_ClosingHashes(t *testing.T) {
	md := "## Title ##\n### C#\n"
	hs := ParseHeadings(md)
	if hs[0].Text != "Title" {
		t.Errorf("closing ## not stripped: %q", hs[0].Text)
	}
	// "C#" 的 # 不是收尾序列（前面非空白），文字保留
	if hs[1].Text != "C#" {
		t.Errorf("C# text should be preserved: %q", hs[1].Text)
	}
	if hs[1].Anchor != "c" {
		t.Errorf("C# anchor should be 'c' (GitHub strips #): %q", hs[1].Anchor)
	}
}

func TestParseHeadings_DedupeAnchors(t *testing.T) {
	md := "## Setup\n## Setup\n## Setup\n"
	hs := ParseHeadings(md)
	want := []string{"setup", "setup-1", "setup-2"}
	for i, h := range hs {
		if h.Anchor != want[i] {
			t.Errorf("anchor[%d] = %q, want %q", i, h.Anchor, want[i])
		}
	}
}

func TestParseHeadings_TooManyHashes(t *testing.T) {
	md := "####### seven\n"
	if hs := ParseHeadings(md); len(hs) != 0 {
		t.Errorf("7 hashes should not be a heading: %+v", hs)
	}
}

// ---- Render ----

func TestRender_Indentation(t *testing.T) {
	hs := []Heading{
		{Level: 2, Text: "Setup", Anchor: "setup"},
		{Level: 3, Text: "Detail", Anchor: "detail"},
		{Level: 2, Text: "Usage", Anchor: "usage"},
	}
	got := Render(hs, 2, 6)
	want := "- [Setup](#setup)\n  - [Detail](#detail)\n- [Usage](#usage)"
	if got != want {
		t.Errorf("Render =\n%q\nwant\n%q", got, want)
	}
}

func TestRender_LevelFilter(t *testing.T) {
	hs := []Heading{
		{Level: 1, Text: "Title", Anchor: "title"},
		{Level: 2, Text: "Setup", Anchor: "setup"},
		{Level: 3, Text: "Detail", Anchor: "detail"},
	}
	// 只要 h2
	got := Render(hs, 2, 2)
	if got != "- [Setup](#setup)" {
		t.Errorf("min=max=2 filter wrong: %q", got)
	}
}

func TestRender_Empty(t *testing.T) {
	if got := Render(nil, 2, 6); got != "" {
		t.Errorf("empty headings → empty string, got %q", got)
	}
	// 全被过滤掉
	hs := []Heading{{Level: 1, Text: "T", Anchor: "t"}}
	if got := Render(hs, 2, 6); got != "" {
		t.Errorf("all filtered → empty, got %q", got)
	}
}

func TestRender_BacktickInLinkText(t *testing.T) {
	hs := []Heading{{Level: 2, Text: "`jdan qr`", Anchor: "jdan-qr"}}
	got := Render(hs, 2, 6)
	if !strings.Contains(got, "[`jdan qr`](#jdan-qr)") {
		t.Errorf("backtick should stay in link text: %q", got)
	}
}

// ---- Insert ----

func TestInsert(t *testing.T) {
	content := "# Doc\n\n" + MarkerStart + "\nOLD\n" + MarkerEnd + "\n\n## Body\n"
	got, err := Insert(content, "- [Body](#body)")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, MarkerStart+"\n- [Body](#body)\n"+MarkerEnd) {
		t.Errorf("insert wrong:\n%s", got)
	}
	if strings.Contains(got, "OLD") {
		t.Error("old content not replaced")
	}
}

func TestInsert_Idempotent(t *testing.T) {
	content := "# Doc\n" + MarkerStart + "\n" + MarkerEnd + "\n"
	toc := "- [A](#a)"
	once, _ := Insert(content, toc)
	twice, _ := Insert(once, toc)
	if once != twice {
		t.Errorf("not idempotent:\n%q\nvs\n%q", once, twice)
	}
}

func TestInsert_MissingMarkers(t *testing.T) {
	if _, err := Insert("# no markers here\n", "- [x](#x)"); err == nil {
		t.Error("missing markers should error")
	}
}

func TestInsert_EmptyToc(t *testing.T) {
	content := MarkerStart + "\nOLD\n" + MarkerEnd
	got, err := Insert(content, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "OLD") {
		t.Error("empty toc should still clear old content")
	}
}
