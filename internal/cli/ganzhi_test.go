package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func runGanzhi(t *testing.T, now time.Time, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newGanzhiCommand(ganzhiCmdDeps{out: &buf, now: now})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

var ganzhiNow = time.Date(2026, time.June, 26, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))

func TestGanzhiCmd_FourPillars(t *testing.T) {
	out, err := runGanzhi(t, ganzhiNow, "1990-05-20", "14:30")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"庚午", "辛巳", "乙酉", "癸未", "路旁土", "立夏"} {
		if !strings.Contains(out, want) {
			t.Errorf("输出缺少 %q:\n%s", want, out)
		}
	}
}

// 不给时刻就不出时柱。默认成 00:00 会凭空造出一个子时，
// 而用户并没有提供那个信息。
func TestGanzhiCmd_NoTimeOmitsHourPillar(t *testing.T) {
	out, err := runGanzhi(t, ganzhiNow, "1990-05-20")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "未提供时刻") {
		t.Errorf("只给日期时应说明时柱为何留空:\n%s", out)
	}
	// 时柱那一行不该出现具体干支
	if strings.Contains(out, "癸未") {
		t.Errorf("没给时刻却算出了时柱:\n%s", out)
	}
	// 也不该显示一个用户没输入的 00:00
	if strings.Contains(out, "00:00") {
		t.Errorf("显示了用户没输入的 00:00，会让人以为是自己输的:\n%s", out)
	}

	// JSON 里 hour 必须显式为 null，不能省略——省略会让消费方分不清
	// 「没给时刻」和「字段名写错了」。
	jsonOut, err := runGanzhi(t, ganzhiNow, "1990-05-20", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &m); err != nil {
		t.Fatalf("--json 应输出合法 JSON: %v", err)
	}
	h, present := m["hour"]
	if !present {
		t.Error("--json 缺少 hour 字段；没给时刻时应为 null 而不是省略")
	}
	if h != nil {
		t.Errorf("没给时刻时 hour 应为 null，实得 %v", h)
	}
}

// 立春前后年柱与月柱都要翻。这是与 lunar 口径分歧的核心。
func TestGanzhiCmd_LichunBoundary(t *testing.T) {
	// 2026 立春 = 02-04 04:02 CST
	before, err := runGanzhi(t, ganzhiNow, "2026-02-04", "03:30")
	if err != nil {
		t.Fatal(err)
	}
	after, err := runGanzhi(t, ganzhiNow, "2026-02-04", "04:30")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(before, "乙巳") {
		t.Errorf("立春前年柱应为乙巳:\n%s", before)
	}
	if !strings.Contains(after, "丙午") {
		t.Errorf("立春后年柱应为丙午:\n%s", after)
	}
}

// 差异期（立春后、春节前）必须主动提示，非差异期不能打这行。
func TestGanzhiCmd_BasisDifferHint(t *testing.T) {
	out, err := runGanzhi(t, ganzhiNow, "2026-02-10", "12:00")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "生肖年") || !strings.Contains(out, "节气年") {
		t.Errorf("差异期应提示两套口径:\n%s", out)
	}

	out, err = runGanzhi(t, ganzhiNow, "2026-06-26", "12:00")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "不同") {
		t.Errorf("非差异期不该打差异提示:\n%s", out)
	}
}

// 子时争议：两派日柱不同，且都要标明用的是哪派。
func TestGanzhiCmd_LateZi(t *testing.T) {
	def, err := runGanzhi(t, ganzhiNow, "2026-07-27", "23:30")
	if err != nil {
		t.Fatal(err)
	}
	late, err := runGanzhi(t, ganzhiNow, "2026-07-27", "23:30", "--late-zi")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(def, "癸卯") {
		t.Errorf("默认派 23:30 日柱应翻到次日（癸卯）:\n%s", def)
	}
	if !strings.Contains(late, "壬寅") {
		t.Errorf("晚子时派 23:30 日柱不该翻（壬寅）:\n%s", late)
	}
	for _, out := range []string{def, late} {
		if !strings.Contains(out, "两派分歧") {
			t.Errorf("23:30 必须标明用的是哪派:\n%s", out)
		}
	}
	// 22:59 不在争议区，不该打这行
	ok, err := runGanzhi(t, ganzhiNow, "2026-07-27", "22:59")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ok, "两派分歧") {
		t.Errorf("22:59 不在争议区，不该提示:\n%s", ok)
	}
}

// --tz 声明挂钟属于哪个时区。给了日期时刻时，挂钟不动、绝对瞬间平移，
// 所以变的是年柱月柱（它们跟节气时刻比），不是日柱时柱。
func TestGanzhiCmd_TZShiftsYearPillarNotDay(t *testing.T) {
	// 2026 立春 = 04:02 CST = 05:02 JST。
	// 挂钟 04:30 在 CST 下已过立春，在 JST 下（=03:30 CST）还没到。
	cst, err := runGanzhi(t, ganzhiNow, "2026-02-04", "04:30")
	if err != nil {
		t.Fatal(err)
	}
	jst, err := runGanzhi(t, ganzhiNow, "2026-02-04", "04:30", "--tz", "Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cst, "丙午") {
		t.Errorf("CST 04:30 年柱应为丙午（立春后）:\n%s", cst)
	}
	if !strings.Contains(jst, "乙巳") {
		t.Errorf("JST 04:30 = CST 03:30，年柱应为乙巳（立春前）:\n%s", jst)
	}
	// 日柱不受影响：两边挂钟同为 2/4 04:30
	if !strings.Contains(cst, "己酉") || !strings.Contains(jst, "己酉") {
		t.Errorf("日柱由本地挂钟定，两边都该是己酉\nCST:\n%s\nJST:\n%s", cst, jst)
	}
}

// 1929-01-01 之前用北京地方平太阳时 UTC+8:05:43。
func TestGanzhiCmd_PreThirtyNineTwentyNineZone(t *testing.T) {
	old, err := runGanzhi(t, ganzhiNow, "1920-06-15", "12:00")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(old, "LMT+8:05:43") {
		t.Errorf("1929 前应用北京地方平时:\n%s", old)
	}
	recent, err := runGanzhi(t, ganzhiNow, "1930-06-15", "12:00")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(recent, "CST") || strings.Contains(recent, "LMT") {
		t.Errorf("1929 起应用 CST:\n%s", recent)
	}
}

func TestGanzhiCmd_OfLookup(t *testing.T) {
	out, err := runGanzhi(t, ganzhiNow, "--of", "甲子")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"甲子", "第 1 个", "海中金", "鼠"} {
		if !strings.Contains(out, want) {
			t.Errorf("--of 甲子 输出缺少 %q:\n%s", want, out)
		}
	}
	// 阴阳不配的组合必须报错，不能静默给个「最接近」的
	if _, err := runGanzhi(t, ganzhiNow, "--of", "甲丑"); err == nil {
		t.Error("「甲丑」阴阳不配，应报错")
	}
	if _, err := runGanzhi(t, ganzhiNow, "--of", "不是干支"); err == nil {
		t.Error("非干支输入应报错")
	}
}

func TestGanzhiCmd_Table(t *testing.T) {
	out, err := runGanzhi(t, ganzhiNow, "--table")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"甲子", "癸亥", "海中金", "大海水"} {
		if !strings.Contains(out, want) {
			t.Errorf("--table 缺少 %q", want)
		}
	}
	// JSON 版必须是 60 条
	jsonOut, err := runGanzhi(t, ganzhiNow, "--table", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Count      int              `json:"count"`
		Sexagenary []map[string]any `json:"sexagenary"`
	}
	if err := json.Unmarshal([]byte(jsonOut), &v); err != nil {
		t.Fatal(err)
	}
	if v.Count != 60 || len(v.Sexagenary) != 60 {
		t.Errorf("--table --json 应有 60 条，实得 count=%d len=%d", v.Count, len(v.Sexagenary))
	}
}

// 模式互斥：--table 与 --of 不能同用，两者都不接位置参数。
func TestGanzhiCmd_ModeConflicts(t *testing.T) {
	if _, err := runGanzhi(t, ganzhiNow, "--table", "--of", "甲子"); err == nil {
		t.Error("--table 与 --of 应互斥")
	}
	if _, err := runGanzhi(t, ganzhiNow, "--table", "2026-01-01"); err == nil {
		t.Error("--table 不该接位置参数")
	}
	if _, err := runGanzhi(t, ganzhiNow, "--of", "甲子", "2026-01-01"); err == nil {
		t.Error("--of 不该接位置参数")
	}
}

func TestGanzhiCmd_BadInput(t *testing.T) {
	for _, args := range [][]string{
		{"not-a-date"},
		{"2026-01-01", "25:00"},
		{"2026-01-01", "12:00", "--tz", "Nowhere/Nothing"},
	} {
		if _, err := runGanzhi(t, ganzhiNow, args...); err == nil {
			t.Errorf("%v 应报错", args)
		}
	}
}

func TestGanzhiCmd_JSON(t *testing.T) {
	out, err := runGanzhi(t, ganzhiNow, "1990-05-20", "14:30", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("--json 应输出合法 JSON: %v\n%s", err, out)
	}
	// 口径必须显式写在 JSON 里，消费方不该靠猜
	if m["ganzhi_basis"] != "solar-term-year" {
		t.Errorf("ganzhi_basis = %v, want solar-term-year", m["ganzhi_basis"])
	}
	for _, k := range []string{"year", "month", "day", "hour"} {
		p, ok := m[k].(map[string]any)
		if !ok {
			t.Fatalf("%s 应为对象，实得 %T", k, m[k])
		}
		for _, f := range []string{"ganzhi", "gan", "zhi", "gan_element", "zhi_element", "yin_yang", "nayin"} {
			if _, ok := p[f]; !ok {
				t.Errorf("%s 缺字段 %s", k, f)
			}
		}
	}
}
