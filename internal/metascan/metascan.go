// Package metascan 解析网页 HTML，抽出 <title> / <meta> / Open Graph / Twitter Card /
// canonical / favicon，并做「分享/SEO 体检」。用 golang.org/x/net/html（已在依赖图，
// 0 新依赖）的正经 tokenizer，不用脆正则。纯函数：喂 HTML 就行，不联网。
package metascan

import (
	"io"
	"strings"

	"golang.org/x/net/html"
)

// Meta 是一个页面的元数据。OG/Twitter 的 key 去掉了 "og:"/"twitter:" 前缀。
type Meta struct {
	Title       string            `json:"title,omitempty"`
	Description string            `json:"description,omitempty"`
	Canonical   string            `json:"canonical,omitempty"`
	Charset     string            `json:"charset,omitempty"`
	Lang        string            `json:"lang,omitempty"`
	Robots      string            `json:"robots,omitempty"`
	Author      string            `json:"author,omitempty"`
	Keywords    string            `json:"keywords,omitempty"`
	Viewport    string            `json:"viewport,omitempty"`
	OG          map[string]string `json:"open_graph,omitempty"`
	Twitter     map[string]string `json:"twitter,omitempty"`
	Icons       []string          `json:"icons,omitempty"`
}

// ParseMeta 解析一段 HTML。畸形 HTML 也能解析（x/net/html 容错）。
func ParseMeta(r io.Reader) (Meta, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return Meta{}, err
	}
	m := Meta{OG: map[string]string{}, Twitter: map[string]string{}}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "html":
				if m.Lang == "" {
					m.Lang = attr(n, "lang")
				}
			case "title":
				if m.Title == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
					m.Title = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				handleMeta(n, &m)
			case "link":
				rel := strings.ToLower(attr(n, "rel"))
				href := attr(n, "href")
				if href == "" {
					break
				}
				if rel == "canonical" && m.Canonical == "" {
					m.Canonical = href
				}
				if strings.Contains(rel, "icon") {
					m.Icons = append(m.Icons, href)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// 清掉空 map，便于 json omitempty
	if len(m.OG) == 0 {
		m.OG = nil
	}
	if len(m.Twitter) == 0 {
		m.Twitter = nil
	}
	return m, nil
}

func handleMeta(n *html.Node, m *Meta) {
	if cs := attr(n, "charset"); cs != "" && m.Charset == "" {
		m.Charset = cs
	}
	content := attr(n, "content")
	if m.Charset == "" && strings.EqualFold(attr(n, "http-equiv"), "content-type") {
		if i := strings.Index(strings.ToLower(content), "charset="); i >= 0 {
			m.Charset = strings.TrimSpace(content[i+len("charset="):])
		}
	}

	name := strings.ToLower(attr(n, "name"))
	prop := strings.ToLower(attr(n, "property"))

	switch {
	case strings.HasPrefix(prop, "og:"):
		putFirst(m.OG, strings.TrimPrefix(prop, "og:"), content)
	case strings.HasPrefix(name, "twitter:"):
		putFirst(m.Twitter, strings.TrimPrefix(name, "twitter:"), content)
	case name == "description" && m.Description == "":
		m.Description = content
	case name == "keywords" && m.Keywords == "":
		m.Keywords = content
	case name == "author" && m.Author == "":
		m.Author = content
	case name == "robots" && m.Robots == "":
		m.Robots = content
	case name == "viewport" && m.Viewport == "":
		m.Viewport = content
	}
}

// putFirst 只保留第一次出现的值（og:image 等重复时取首个）。
func putFirst(m map[string]string, k, v string) {
	if k == "" {
		return
	}
	if _, ok := m[k]; !ok {
		m[k] = v
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

// Issue 是一条体检结果。
type Issue struct {
	Level string `json:"level"` // warn | info
	Msg   string `json:"msg"`
}

// Audit 做「分享/SEO 体检」，指出缺哪些关键标签。
func Audit(m Meta) []Issue {
	var out []Issue
	if m.Title == "" && m.OG["title"] == "" {
		out = append(out, Issue{"warn", "缺 <title> 和 og:title → 分享/搜索无标题"})
	}
	if m.Description == "" && m.OG["description"] == "" {
		out = append(out, Issue{"warn", "缺 description 和 og:description → 分享/SEO 无摘要"})
	}
	if m.OG["image"] == "" {
		out = append(out, Issue{"warn", "缺 og:image → 分享卡片可能没缩略图"})
	}
	if m.Canonical == "" {
		out = append(out, Issue{"info", "缺 canonical → 多 URL 收录可能分散权重"})
	}
	return out
}
