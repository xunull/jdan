package shuangpinx

import "testing"

func enc(t *testing.T, schemeID, py string) string {
	t.Helper()
	s, ok := Get(schemeID)
	if !ok {
		t.Fatalf("方案 %q 不存在", schemeID)
	}
	return s.Encode(py)
}

// 各方案对同一音节的编码（照 RIME 各 schema 规则手算核对）。
func TestEncode_PerScheme(t *testing.T) {
	cases := []struct {
		scheme, py, want string
	}{
		// 中 = zhong（声母 zh + 韵母 ong）
		{"flypy", "zhong", "vs"}, {"ziranma", "zhong", "vs"}, {"mspy", "zhong", "vs"},
		{"abc", "zhong", "as"}, {"pyjj", "zhong", "vy"},
		// 文 = wen（w + en）
		{"flypy", "wen", "wf"}, {"ziranma", "wen", "wf"}, {"mspy", "wen", "wf"},
		{"abc", "wen", "wf"}, {"pyjj", "wen", "wr"},
		// dan（d + an）
		{"flypy", "dan", "dj"}, {"ziranma", "dan", "dj"}, {"mspy", "dan", "dj"},
		{"abc", "dan", "dj"}, {"pyjj", "dan", "df"},
		// 双 = shuang（sh + uang）
		{"flypy", "shuang", "ul"}, {"ziranma", "shuang", "ud"}, {"abc", "shuang", "vt"},
		// 微软 ing 落在分号键
		{"mspy", "ping", "p;"},
	}
	for _, c := range cases {
		if got := enc(t, c.scheme, c.py); got != c.want {
			t.Errorf("[%s] Encode(%q) = %q, want %q", c.scheme, c.py, got, c.want)
		}
	}
}

// 小鹤零声母（首字母倍写规则）。
func TestEncode_FlypyZeroInitial(t *testing.T) {
	cases := map[string]string{"an": "aj", "ai": "ad", "ao": "ac", "e": "ee", "ou": "oz", "ang": "ah"}
	for py, want := range cases {
		if got := enc(t, "flypy", py); got != want {
			t.Errorf("小鹤零声母 Encode(%q) = %q, want %q", py, got, want)
		}
	}
}

func TestValid(t *testing.T) {
	s := Default()
	if code, ok := s.Valid("dan"); !ok || code != "dj" {
		t.Errorf("dan 应合法 dj: %q,%v", code, ok)
	}
	for _, bad := range []string{"xyz", "b", ""} {
		if _, ok := s.Valid(bad); ok {
			t.Errorf("%q 不应合法（非 2 键）", bad)
		}
	}
}

func TestGet_NamesAndAliases(t *testing.T) {
	if s, ok := Get("小鹤"); !ok || s.ID != "flypy" {
		t.Errorf("中文名 小鹤 应解析到 flypy")
	}
	if s, ok := Get("sogou"); !ok || s.ID != "mspy" {
		t.Errorf("搜狗别名应归到 微软(mspy)")
	}
	if s, ok := Get("flypy"); !ok || s.Name != "小鹤" {
		t.Errorf("id flypy 应解析")
	}
	if _, ok := Get("nope"); ok {
		t.Error("未知方案应 false")
	}
}

func TestAll_DefaultFirst(t *testing.T) {
	all := All()
	if len(all) != 5 || all[0].ID != "flypy" {
		t.Errorf("应 5 套、小鹤在首: %d", len(all))
	}
	if Default().ID != "flypy" || Flypy().ID != "flypy" {
		t.Error("默认/Flypy 应为小鹤")
	}
}
