package ganzhix

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xunull/jdan/internal/jieqix"
)

var cst = time.FixedZone("CST", 8*3600)

func readTSV(t *testing.T, name string) [][]string {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("打不开锚点表 %s: %v", name, err)
	}
	defer f.Close()
	var rows [][]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rows = append(rows, strings.Split(line, "\t"))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("读 %s: %v", name, err)
	}
	return rows
}

// 日柱相位的权威验证：拿 testdata/day_pillar.tsv 的 13 条锚点逐条比对。
// 其中 2 条标 [权威]（直接来自外部陈述），11 条标 [推导]（沿 60 日循环推出）。
func TestDayPillar_AgainstAnchors(t *testing.T) {
	rows := readTSV(t, "day_pillar.tsv")
	if len(rows) != 13 {
		t.Fatalf("day_pillar.tsv 应有 13 行，实得 %d", len(rows))
	}
	authoritative := 0
	for _, r := range rows {
		day, err := time.Parse("2006-01-02", r[0])
		if err != nil {
			t.Fatalf("日期列 %q: %v", r[0], err)
		}
		wantGZ, wantIdx := r[1], r[2]
		kind := r[3]
		if kind == "[权威]" {
			authoritative++
		}

		// 取当天正午，避开日界与子时争议——这条测的是日柱相位，不是边界。
		at := time.Date(day.Year(), day.Month(), day.Day(), 12, 0, 0, 0, cst)
		fp, err := Of(at, Options{})
		if err != nil {
			t.Fatalf("Of(%s): %v", r[0], err)
		}
		if fp.Day.String() != wantGZ || strconv.Itoa(fp.Day.Index) != wantIdx {
			t.Errorf("%s 日柱 = %s(idx %d), 锚点 = %s(idx %s)  %s %s",
				r[0], fp.Day, fp.Day.Index, wantGZ, wantIdx, kind, r[4])
		}
	}
	if authoritative != 2 {
		t.Errorf("锚点表里 [权威] 行应有 2 条，实得 %d —— 相位的外部依据被改动了？", authoritative)
	}
}

// 五虎遁：甲己之年丙作首，乙庚之岁戊为头，丙辛必定寻庚起，
// 丁壬壬位顺行流，戊癸何方觅，甲寅之上好追求。
//
// 把口诀本身写成断言。实现里用的是公式 (年干×2+2) mod 10，
// 公式对不对由口诀验，而不是反过来。
func TestMonthPillar_WuHuDun(t *testing.T) {
	want := map[string]string{ // 年干 -> 寅月的天干
		"甲": "丙", "己": "丙",
		"乙": "戊", "庚": "戊",
		"丙": "庚", "辛": "庚",
		"丁": "壬", "壬": "壬",
		"戊": "甲", "癸": "甲",
	}
	for gi, gan := range TianGan {
		got := TianGan[(gi*2+2)%10]
		if got != want[gan] {
			t.Errorf("五虎遁：%s 年寅月天干 = %s, 口诀说 %s", gan, got, want[gan])
		}
	}
}

// 五鼠遁：甲己还加甲，乙庚丙作初，丙辛从戊起，丁壬庚子居，戊癸何方发，壬子是真途。
func TestHourPillar_WuShuDun(t *testing.T) {
	want := map[string]string{ // 日干 -> 子时的天干
		"甲": "甲", "己": "甲",
		"乙": "丙", "庚": "丙",
		"丙": "戊", "辛": "戊",
		"丁": "庚", "壬": "庚",
		"戊": "壬", "癸": "壬",
	}
	for gi, gan := range TianGan {
		got := TianGan[(gi*2)%10]
		if got != want[gan] {
			t.Errorf("五鼠遁：%s 日子时天干 = %s, 口诀说 %s", gan, got, want[gan])
		}
	}
}

// 年柱以立春为界，不是以正月初一为界。
// 这是与 lunarx 口径分歧的核心，也是最容易被写错成 t.Year() 的地方。
func TestYearPillar_LichunBoundary(t *testing.T) {
	lichun, err := lichunOf(2026)
	if err != nil {
		t.Fatal(err)
	}
	before := lichun.In(cst).Add(-2 * time.Minute)
	after := lichun.In(cst).Add(2 * time.Minute)

	fpB, err := Of(before, Options{})
	if err != nil {
		t.Fatal(err)
	}
	fpA, err := Of(after, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if fpB.TermYear != 2025 {
		t.Errorf("立春前 2 分钟 TermYear = %d, want 2025", fpB.TermYear)
	}
	if fpA.TermYear != 2026 {
		t.Errorf("立春后 2 分钟 TermYear = %d, want 2026", fpA.TermYear)
	}
	if fpB.Year.String() == fpA.Year.String() {
		t.Errorf("立春前后年柱都是 %s —— 立春分界没起作用", fpB.Year)
	}
	// 立春前后月柱也必须翻（丑月 → 寅月）
	if fpB.Month.Zhi != "丑" || fpA.Month.Zhi != "寅" {
		t.Errorf("立春前后月支 = %s -> %s, want 丑 -> 寅", fpB.Month.Zhi, fpA.Month.Zhi)
	}
	if fpA.MonthTerm != "立春" {
		t.Errorf("立春后 MonthTerm = %s, want 立春", fpA.MonthTerm)
	}
}

// 元旦到立春之间：年柱还是上一年的。这段每年约 34 天，
// 是「生肖年 vs 节气年」分歧最直观的体现。
func TestYearPillar_NewYearGap(t *testing.T) {
	// 2026 春节是 2026-02-17，立春是 2026-02-04（北京时）。
	// 所以 2/4 到 2/16 这段，生肖年还是乙巳（蛇），节气年已是丙午（马）。
	at := time.Date(2026, time.February, 10, 12, 0, 0, 0, cst)
	fp, err := Of(at, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if fp.TermYear != 2026 || fp.Year.String() != "丙午" {
		t.Errorf("2026-02-10 节气年 = %d %s, want 2026 丙午", fp.TermYear, fp.Year)
	}
	// 元旦：还在上一个节气年
	at = time.Date(2026, time.January, 1, 12, 0, 0, 0, cst)
	fp, err = Of(at, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if fp.TermYear != 2025 {
		t.Errorf("2026-01-01 节气年 = %d, want 2025", fp.TermYear)
	}
}

// 子时分歧：23:00–23:59 两派给出不同的日柱与时柱。
func TestZiHourDispute(t *testing.T) {
	at := time.Date(2026, time.July, 27, 23, 30, 0, 0, cst)

	def, err := Of(at, Options{})
	if err != nil {
		t.Fatal(err)
	}
	late, err := Of(at, Options{LateZi: true})
	if err != nil {
		t.Fatal(err)
	}

	// 锚点：2026-07-27 = 壬寅日，2026-07-28 = 癸卯日
	if def.Day.String() != "癸卯" {
		t.Errorf("默认派 23:30 日柱 = %s, want 癸卯（已翻到 7/28）", def.Day)
	}
	if late.Day.String() != "壬寅" {
		t.Errorf("晚子时派 23:30 日柱 = %s, want 壬寅（不翻）", late.Day)
	}
	// 两派时支都是子，但时干不同（各按自己的日干五鼠遁）
	if def.Hour.Zhi != "子" || late.Hour.Zhi != "子" {
		t.Errorf("23:30 时支 = %s / %s, 两派都该是子", def.Hour.Zhi, late.Hour.Zhi)
	}
	if def.Hour.Gan == late.Hour.Gan {
		t.Errorf("两派时干都是 %s —— 五鼠遁没有跟着各自的日干走", def.Hour.Gan)
	}
	if !def.ZiDisputed || !late.ZiDisputed {
		t.Error("23:30 应标记 ZiDisputed")
	}

	// 22:59 不在争议区，两派必须一致
	before := time.Date(2026, time.July, 27, 22, 59, 0, 0, cst)
	d2, _ := Of(before, Options{})
	l2, _ := Of(before, Options{LateZi: true})
	if d2.Day.String() != l2.Day.String() || d2.Hour.String() != l2.Hour.String() {
		t.Errorf("22:59 两派应一致，实得 %s%s vs %s%s", d2.Day, d2.Hour, l2.Day, l2.Hour)
	}
	if d2.ZiDisputed {
		t.Error("22:59 不该标 ZiDisputed")
	}
}

// 十二时辰的每个边界。23 点和 0 点必须归到同一辰（子）。
func TestHourPillar_ShichenBoundaries(t *testing.T) {
	want := []string{
		"子", "丑", "丑", "寅", "寅", "卯", "卯", "辰", "辰", "巳", "巳", "午",
		"午", "未", "未", "申", "申", "酉", "酉", "戌", "戌", "亥", "亥", "子",
	}
	for h := range 24 {
		at := time.Date(2026, time.June, 15, h, 30, 0, 0, cst)
		fp, err := Of(at, Options{LateZi: true}) // 用晚子时派，避免 23 点翻日干扰对照
		if err != nil {
			t.Fatal(err)
		}
		if fp.Hour.Zhi != want[h] {
			t.Errorf("%02d:30 时支 = %s, want %s", h, fp.Hour.Zhi, want[h])
		}
	}
}

func TestPillarAttributes(t *testing.T) {
	p := FromIndex(0) // 甲子
	if p.String() != "甲子" || p.Element != "木" || p.ZhiElem != "水" ||
		p.YinYang != "阳" || p.Zodiac != "鼠" || p.Nayin() != "海中金" {
		t.Errorf("甲子 = %+v, nayin=%s", p, p.Nayin())
	}
	p = FromIndex(59) // 癸亥
	if p.String() != "癸亥" || p.Element != "水" || p.YinYang != "阴" ||
		p.Zodiac != "猪" || p.Nayin() != "大海水" {
		t.Errorf("癸亥 = %+v, nayin=%s", p, p.Nayin())
	}
}

func TestSexagenary(t *testing.T) {
	all := Sexagenary()
	if len(all) != 60 {
		t.Fatalf("六十甲子应有 60 项，实得 %d", len(all))
	}
	if all[0].String() != "甲子" || all[59].String() != "癸亥" {
		t.Errorf("首尾 = %s..%s, want 甲子..癸亥", all[0], all[59])
	}
	// 60 个全不重复，且阴阳交替
	seen := map[string]bool{}
	for i, p := range all {
		if seen[p.String()] {
			t.Errorf("第 %d 项 %s 重复", i, p)
		}
		seen[p.String()] = true
		wantYY := "阳"
		if i%2 == 1 {
			wantYY = "阴"
		}
		if p.YinYang != wantYY {
			t.Errorf("%s (idx %d) 阴阳 = %s, want %s", p, i, p.YinYang, wantYY)
		}
	}
	// 纳音：30 组，每组恰好 2 个干支
	count := map[string]int{}
	for _, p := range all {
		count[p.Nayin()]++
	}
	if len(count) != 30 {
		t.Errorf("纳音应有 30 种，实得 %d", len(count))
	}
	for n, c := range count {
		if c != 2 {
			t.Errorf("纳音「%s」对应 %d 个干支，应为 2", n, c)
		}
	}
}

// 「甲丑」这类阴阳不配的组合必须被挡掉——120 个「看起来像」的组合里
// 只有 60 个是合法的。
func TestIndexOf(t *testing.T) {
	for i, p := range Sexagenary() {
		got, ok := IndexOf(p.String())
		if !ok || got != i {
			t.Errorf("IndexOf(%s) = %d,%v, want %d,true", p, got, ok, i)
		}
	}
	for _, bad := range []string{"甲丑", "乙子", "甲", "甲子丙", "", "abc", "子甲"} {
		if _, ok := IndexOf(bad); ok {
			t.Errorf("IndexOf(%q) 应判为非法", bad)
		}
	}
}

func TestFromGanZhi_PanicsOnMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("甲(0) 配丑(1) 阴阳不配，应 panic 而不是静默返回")
		}
	}()
	fromGanZhi(0, 1)
}

// 与 jieqix 的月令一致性：月支必须跟着 PrevMajor 走，不跟公历月走。
func TestMonthPillar_FollowsSolarTerm(t *testing.T) {
	for _, at := range []time.Time{
		time.Date(2026, time.January, 3, 12, 0, 0, 0, cst), // 前一个节在 2025 年
		time.Date(2026, time.March, 10, 12, 0, 0, 0, cst),
		time.Date(1900, time.August, 20, 12, 0, 0, 0, cst),
		time.Date(2100, time.November, 1, 12, 0, 0, 0, cst),
	} {
		fp, err := Of(at, Options{})
		if err != nil {
			t.Fatalf("Of(%s): %v", at.Format("2006-01-02"), err)
		}
		major, err := jieqix.PrevMajor(at)
		if err != nil {
			t.Fatal(err)
		}
		if fp.MonthTerm != major.Name {
			t.Errorf("%s MonthTerm = %s, PrevMajor = %s",
				at.Format("2006-01-02"), fp.MonthTerm, major.Name)
		}
		wantZhi := DiZhi[(2+major.Index/2)%12]
		if fp.Month.Zhi != wantZhi {
			t.Errorf("%s 月支 = %s, 由「%s」应得 %s",
				at.Format("2006-01-02"), fp.Month.Zhi, major.Name, wantZhi)
		}
	}
}

// FromIndex 的文档承诺「传负数或超过 59 都不会 panic」——承诺了就得验。
// 干支是循环的，越界归一是正确行为而不是容错兜底。
func TestFromIndex_Normalizes(t *testing.T) {
	base := FromIndex(0)
	for _, i := range []int{-60, -1, 60, 61, 119, 600, -601} {
		got := FromIndex(i)
		want := FromIndex(((i % 60) + 60) % 60)
		if got != want {
			t.Errorf("FromIndex(%d) = %s(idx %d), want %s(idx %d)",
				i, got, got.Index, want, want.Index)
		}
		if got.Index < 0 || got.Index > 59 {
			t.Errorf("FromIndex(%d) 归一后 Index = %d，越界", i, got.Index)
		}
	}
	if FromIndex(-60) != base || FromIndex(60) != base {
		t.Error("±60 应回到甲子")
	}
	if FromIndex(-1).String() != "癸亥" {
		t.Errorf("FromIndex(-1) = %s, want 癸亥", FromIndex(-1))
	}
}
