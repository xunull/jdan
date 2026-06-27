package metascan

import (
	"strings"
	"testing"
)

const fullHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>  测试页  </title>
  <meta name="description" content="一段描述">
  <meta name="robots" content="index,follow">
  <meta property="og:title" content="OG标题">
  <meta property="og:image" content="https://x.com/a.png">
  <meta property="og:type" content="article">
  <meta name="twitter:card" content="summary_large_image">
  <link rel="canonical" href="https://x.com/p">
  <link rel="icon" href="/favicon.ico">
</head>
<body>hi</body></html>`

func TestParseMeta_Full(t *testing.T) {
	m, err := ParseMeta(strings.NewReader(fullHTML))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "测试页" { // 应 trim 空白
		t.Errorf("title=%q", m.Title)
	}
	if m.Lang != "zh-CN" || m.Charset != "utf-8" {
		t.Errorf("lang=%q charset=%q", m.Lang, m.Charset)
	}
	if m.Description != "一段描述" || m.Robots != "index,follow" {
		t.Errorf("desc=%q robots=%q", m.Description, m.Robots)
	}
	if m.OG["title"] != "OG标题" || m.OG["image"] != "https://x.com/a.png" || m.OG["type"] != "article" {
		t.Errorf("og=%v", m.OG)
	}
	if m.Twitter["card"] != "summary_large_image" {
		t.Errorf("twitter=%v", m.Twitter)
	}
	if m.Canonical != "https://x.com/p" {
		t.Errorf("canonical=%q", m.Canonical)
	}
	if len(m.Icons) != 1 || m.Icons[0] != "/favicon.ico" {
		t.Errorf("icons=%v", m.Icons)
	}
}

// 畸形 HTML（缺 head/body、标签未闭合）也应能解析（x/net/html 容错，会自动补全树）。
func TestParseMeta_Malformed(t *testing.T) {
	const broken = `<meta name="description" content="d"><title>标题</title><p>正文`
	m, err := ParseMeta(strings.NewReader(broken))
	if err != nil {
		t.Fatal(err)
	}
	if m.Description != "d" || m.Title != "标题" {
		t.Errorf("畸形 HTML 应仍抽到 desc/title，got desc=%q title=%q", m.Description, m.Title)
	}
}

// 重复 og:image 取第一个。
func TestParseMeta_DuplicateOG(t *testing.T) {
	const dup = `<html><head>
		<meta property="og:image" content="first">
		<meta property="og:image" content="second"></head></html>`
	m, _ := ParseMeta(strings.NewReader(dup))
	if m.OG["image"] != "first" {
		t.Errorf("应取首个 og:image，got %q", m.OG["image"])
	}
}

// http-equiv content-type 里的 charset 也要识别。
func TestParseMeta_HTTPEquivCharset(t *testing.T) {
	const h = `<html><head><meta http-equiv="Content-Type" content="text/html; charset=gbk"></head></html>`
	m, _ := ParseMeta(strings.NewReader(h))
	if m.Charset != "gbk" {
		t.Errorf("charset=%q，应为 gbk", m.Charset)
	}
}

func TestParseMeta_EmptyMapsNil(t *testing.T) {
	m, _ := ParseMeta(strings.NewReader(`<html><head><title>x</title></head></html>`))
	if m.OG != nil || m.Twitter != nil {
		t.Errorf("无 og/twitter 时 map 应为 nil（json omitempty），og=%v tw=%v", m.OG, m.Twitter)
	}
}

func TestAudit_Missing(t *testing.T) {
	issues := Audit(Meta{Title: "只有标题"})
	var msgs string
	for _, is := range issues {
		msgs += is.Msg + "\n"
	}
	for _, want := range []string{"og:description", "og:image", "canonical"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("体检应提到 %q，实际:\n%s", want, msgs)
		}
	}
}

func TestAudit_Complete(t *testing.T) {
	m := Meta{
		Title:       "t",
		Description: "d",
		Canonical:   "https://x/p",
		OG:          map[string]string{"image": "https://x/i.png"},
	}
	if got := Audit(m); len(got) != 0 {
		t.Errorf("关键标签齐全应无体检项，got %v", got)
	}
}
