package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// jieqiNow 是本文件自己的测试时刻。
//
// 不借用 ganzhi_test.go 里的同类变量：那会让本提交依赖后一个提交，
// git bisect 走到这里会「编译过但测试挂」。测试夹具跨文件共享省下的
// 几行，代价是二分历史断一节。
var jieqiNow = time.Date(2026, time.June, 26, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))

func runJieqi(t *testing.T, now time.Time, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := newJieqiCommand(jieqiCmdDeps{out: &buf, now: now})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestJieqiCmd_Year(t *testing.T) {
	out, err := runJieqi(t, jieqiNow, "2026")
	if err != nil {
		t.Fatal(err)
	}
	// 锚点（北京时）：小寒 01-05 16:23、立春 02-04 04:02、冬至 12-22 04:50
	for _, want := range []string{
		"2026 年二十四节气",
		"01-05 16:23", "小寒", "285°",
		"02-04 04:02", "立春", "315°",
		"12-22 04:50", "冬至", "270°",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少 %q:\n%s", want, out)
		}
	}
	// 24 行 + 标题
	if n := strings.Count(out, "°"); n != 24 {
		t.Errorf("应有 24 个节气，实得 %d 行:\n%s", n, out)
	}
}

// 跨日的两个：雨水与芒种在日本时是次日凌晨，转北京时要退回前一天。
// 这类边界最容易在时区转换里被写错。
func TestJieqiCmd_CrossDayTerms(t *testing.T) {
	out, err := runJieqi(t, jieqiNow, "2026")
	if err != nil {
		t.Fatal(err)
	}
	// NAOJ 日本时：雨水 02-19 00:52 → 北京时 02-18 23:51
	//              芒种 06-06 00:48 → 北京时 06-05 23:48
	for _, want := range []string{"02-18 23:5", "06-05 23:4"} {
		if !strings.Contains(out, want) {
			t.Errorf("跨日节气时刻不对，缺 %q:\n%s", want, out)
		}
	}
}

// 默认年份取「今天」所在的年。
func TestJieqiCmd_DefaultYear(t *testing.T) {
	out, err := runJieqi(t, jieqiNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2026 年二十四节气") {
		t.Errorf("默认应为 now 的年份:\n%s", out)
	}
}

func TestJieqiCmd_Next(t *testing.T) {
	out, err := runJieqi(t, jieqiNow, "--next")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-06-26 之后的下一个节气是小暑（07-07 09:56 CST）
	if !strings.Contains(out, "小暑") || !strings.Contains(out, "还有") {
		t.Errorf("--next 输出不对:\n%s", out)
	}
}

// --tz 对 jieqi 只改显示，不改答案：换时区后节气的顺序与个数必须一致，
// 且同一个节气对应的是同一个物理瞬间。
func TestJieqiCmd_TZIsDisplayOnly(t *testing.T) {
	cstOut, err := runJieqi(t, jieqiNow, "2026", "--json")
	if err != nil {
		t.Fatal(err)
	}
	tokyoOut, err := runJieqi(t, jieqiNow, "2026", "--tz", "Asia/Tokyo", "--json")
	if err != nil {
		t.Fatal(err)
	}
	type term struct {
		Name string `json:"name"`
		Time string `json:"time"`
	}
	var a, b struct {
		Terms []term `json:"terms"`
	}
	if err := json.Unmarshal([]byte(cstOut), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(tokyoOut), &b); err != nil {
		t.Fatal(err)
	}
	if len(a.Terms) != 24 || len(b.Terms) != 24 {
		t.Fatalf("两边都该是 24 个，实得 %d / %d", len(a.Terms), len(b.Terms))
	}
	for i := range a.Terms {
		if a.Terms[i].Name != b.Terms[i].Name {
			t.Errorf("第 %d 个节气名不一致：%s vs %s", i, a.Terms[i].Name, b.Terms[i].Name)
		}
		ta, err := time.Parse(time.RFC3339, a.Terms[i].Time)
		if err != nil {
			t.Fatal(err)
		}
		tb, err := time.Parse(time.RFC3339, b.Terms[i].Time)
		if err != nil {
			t.Fatal(err)
		}
		if !ta.Equal(tb) {
			t.Errorf("%s 换时区后指向了不同的瞬间：%s vs %s",
				a.Terms[i].Name, ta.UTC(), tb.UTC())
		}
	}
}

// 超出已验证区间必须提示。不提示的话，工具就在默默把不可信的数字当可信的发。
func TestJieqiCmd_UnverifiedYearWarns(t *testing.T) {
	out, err := runJieqi(t, jieqiNow, "2200")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "超出已验证区间") {
		t.Errorf("2200 年应提示未验证:\n%s", out)
	}
	// 区间内不该打这行
	out, err = runJieqi(t, jieqiNow, "2026")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "超出已验证区间") {
		t.Errorf("2026 在区间内，不该提示:\n%s", out)
	}

	// JSON 里也要有 verified 字段供脚本判断
	jsonOut, err := runJieqi(t, jieqiNow, "2200", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &m); err != nil {
		t.Fatal(err)
	}
	if m["verified"] != false {
		t.Errorf("2200 年 verified 应为 false，实得 %v", m["verified"])
	}
}

func TestJieqiCmd_1929Zone(t *testing.T) {
	out, err := runJieqi(t, jieqiNow, "1920")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "LMT+8:05:43") {
		t.Errorf("1929 前应用北京地方平时:\n%s", out)
	}
}

func TestJieqiCmd_JSON(t *testing.T) {
	out, err := runJieqi(t, jieqiNow, "2026", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Year     int  `json:"year"`
		Verified bool `json:"verified"`
		Terms    []struct {
			Name      string `json:"name"`
			Longitude int    `json:"longitude"`
			Major     bool   `json:"major"`
			Time      string `json:"time"`
		} `json:"terms"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json 应输出合法 JSON: %v", err)
	}
	if v.Year != 2026 || !v.Verified || len(v.Terms) != 24 {
		t.Errorf("json 头部不对: year=%d verified=%v terms=%d", v.Year, v.Verified, len(v.Terms))
	}
	majors := 0
	for _, x := range v.Terms {
		if x.Major {
			majors++
			if (x.Longitude-315+360)%30 != 0 {
				t.Errorf("「%s」标为节但黄经 %d° 不是 315+30k", x.Name, x.Longitude)
			}
		}
	}
	if majors != 12 {
		t.Errorf("「节」应有 12 个，实得 %d", majors)
	}
}

func TestJieqiCmd_BadInput(t *testing.T) {
	if _, err := runJieqi(t, jieqiNow, "not-a-year"); err == nil {
		t.Error("非数字年份应报错")
	}
	if _, err := runJieqi(t, jieqiNow, "2026", "--tz", "Nowhere/Nothing"); err == nil {
		t.Error("非法时区应报错")
	}
}
