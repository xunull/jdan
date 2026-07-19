package termx

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestHumanBytes(t *testing.T) {
	cases := map[uint64]string{
		0:             "0B",
		1023:          "1023B",
		1024:          "1.0Ki",
		1536:          "1.5Ki",
		14 * 1 << 30:  "14Gi",
		1<<40 + 1<<39: "1.5Ti",
		926 * 1 << 30: "926Gi",
	}
	for n, want := range cases {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %s, want %s", n, got, want)
		}
	}
}

// HumanBytes1 在 ≥10 时仍保留一位小数，这正是它相对 HumanBytes 存在的理由：
// 排行榜里 31.2Gi 和 24.8Gi 被压成 31Gi/25Gi 就看不出差距了。
func TestHumanBytes1_KeepsDecimalAbove10(t *testing.T) {
	cases := map[uint64]string{
		0:             "0B",
		1023:          "1023B",
		1024:          "1.0Ki",
		1536:          "1.5Ki",
		14 * 1 << 30:  "14.0Gi",
		926 * 1 << 30: "926.0Gi",
		92650000000:   "86.3Gi", // 设计文档示例：~/Library
		33500000000:   "31.2Gi", // 设计文档示例：Caches
		26600000000:   "24.8Gi", // 设计文档示例：Containers
		1<<40 + 1<<39: "1.5Ti",
	}
	for n, want := range cases {
		if got := HumanBytes1(n); got != want {
			t.Errorf("HumanBytes1(%d) = %s, want %s", n, got, want)
		}
	}
	// 与 HumanBytes 的差异必须真实存在，否则这个函数没有理由
	if HumanBytes(14*1<<30) == HumanBytes1(14*1<<30) {
		t.Error("HumanBytes1 应在 ≥10 时保留小数，与 HumanBytes 不同")
	}
}

func TestBar(t *testing.T) {
	if b := Bar(0, 9); b != strings.Repeat("░", 9) {
		t.Errorf("Bar(0) = %q", b)
	}
	if b := Bar(100, 9); b != strings.Repeat("█", 9) {
		t.Errorf("Bar(100) = %q", b)
	}
	// 50% × 9 → round(4.5)=5 满格
	if filled := strings.Count(Bar(50, 9), "█"); filled != 5 {
		t.Errorf("Bar(50,9) filled = %d, want 5", filled)
	}
	// 越界钳制
	if b := Bar(-10, 5); b != strings.Repeat("░", 5) {
		t.Errorf("Bar(-10,5) 应钳到 0: %q", b)
	}
	if b := Bar(300, 5); b != strings.Repeat("█", 5) {
		t.Errorf("Bar(300,5) 应钳到 100: %q", b)
	}
}

func TestColorize(t *testing.T) {
	if s := Colorize("98%", 98, true); !strings.Contains(s, "\x1b[31m") {
		t.Error("≥90% 应染红")
	}
	if s := Colorize("80%", 80, true); !strings.Contains(s, "\x1b[33m") {
		t.Error("≥75% 应染黄")
	}
	if s := Colorize("50%", 50, true); strings.Contains(s, "\x1b") {
		t.Error("<75% 不染色")
	}
	if s := Colorize("98%", 98, false); strings.Contains(s, "\x1b") {
		t.Error("color=false 不应有 ANSI")
	}
}

func TestVisWidth_IgnoresANSI(t *testing.T) {
	if w := VisWidth("\x1b[31m86%\x1b[0m"); w != 3 {
		t.Errorf("ANSI-wrapped '86%%' visual width = %d, want 3", w)
	}
}

// 回归：CJK locale 下 runewidth 默认把 █ 判成 2 列（ambiguous→wide），与终端渲染（1 列）
// 不符，导致整列错位。termx 用 narrow 条件锁死按 1 列算；中文宽字符仍按 2 列。
// 这条用例是 termx 存在的硬理由：任何重写都会踩回这个坑。
func TestVisWidth_AmbiguousBlocksNarrowUnderCJK(t *testing.T) {
	old := runewidth.DefaultCondition.EastAsianWidth
	runewidth.DefaultCondition.EastAsianWidth = true // 模拟 zh_CN.UTF-8 终端
	defer func() { runewidth.DefaultCondition.EastAsianWidth = old }()

	if w := VisWidth("█"); w != 1 {
		t.Errorf("█ (U+2588) 应按 1 列测量，得到 %d（CJK locale 回归）", w)
	}
	if w := VisWidth("░"); w != 1 {
		t.Errorf("░ (U+2591) 应按 1 列测量，得到 %d", w)
	}
	if w := VisWidth("容"); w != 2 {
		t.Errorf("中文宽字符「容」应仍按 2 列，得到 %d", w)
	}
}

func TestTruncMiddle(t *testing.T) {
	s := "com.apple.TimeMachine.2026-06-27-064158.local@/dev/disk3s5"
	out := TruncMiddle(s, 20)
	if VisWidth(out) > 20 {
		t.Errorf("截断后宽度 %d > 20: %q", VisWidth(out), out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("应含省略号: %q", out)
	}
	if !strings.HasPrefix(out, "com") {
		t.Errorf("应保留头部: %q", out)
	}
	if !strings.HasSuffix(out, "disk3s5") {
		t.Errorf("应保留尾部: %q", out)
	}
	if TruncMiddle("short", 20) != "short" {
		t.Error("未超宽应原样返回")
	}
	if TruncMiddle("anything", 0) != "" {
		t.Error("maxW<=0 应返回空串")
	}
	if TruncMiddle("anything", 1) != "…" {
		t.Error("maxW==1 应只返回省略号")
	}
}

// 中文名截断不能把宽字符切成半个，且结果宽度不超预算。
func TestTruncMiddle_CJK(t *testing.T) {
	s := "应用程序支持目录缓存文件夹"
	for _, w := range []int{4, 7, 10, 13} {
		out := TruncMiddle(s, w)
		if VisWidth(out) > w {
			t.Errorf("TruncMiddle(CJK, %d) 宽度 %d 超预算: %q", w, VisWidth(out), out)
		}
	}
}

func TestTable_RightAlign(t *testing.T) {
	header := []string{"名称", "大小"}
	rows := [][]string{{"a", "1B"}, {"bbbb", "1023B"}}
	out := Table(header, rows, map[int]bool{1: true})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("应有表头 + 2 行，得到 %d 行:\n%s", len(lines), out)
	}
	// 右对齐列的右缘应对齐：每行去掉尾部空格后长度一致
	if !strings.HasSuffix(lines[1], "   1B") {
		t.Errorf("右对齐列应右缘对齐: %q", lines[1])
	}
	// nil rightAlign 全部左对齐，不 panic
	if out := Table(header, rows, nil); !strings.Contains(out, "1B") {
		t.Errorf("rightAlign=nil 应正常渲染:\n%s", out)
	}
}

func TestPad(t *testing.T) {
	if got := Pad("ab", 5, false); got != "ab   " {
		t.Errorf("左对齐 Pad = %q", got)
	}
	if got := Pad("ab", 5, true); got != "   ab" {
		t.Errorf("右对齐 Pad = %q", got)
	}
	if got := Pad("abcdef", 3, false); got != "abcdef" {
		t.Errorf("超宽不截断，应原样返回: %q", got)
	}
	// 宽字符按可见宽度算，不是字节数也不是 rune 数
	if got := Pad("容", 4, false); got != "容  " {
		t.Errorf("宽字符 Pad 应按 2 列算: %q", got)
	}
}
