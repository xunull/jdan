package commitlint

import (
	"strings"
	"testing"
)

// 收集违规规则名，便于断言。
func rules(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Rule
	}
	return out
}

func has(vs []Violation, rule string) bool {
	for _, v := range vs {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

func TestLint_Valid(t *testing.T) {
	cases := []string{
		"feat: add pagination",
		"fix(api): handle nil cursor",
		"docs: update README",
		"feat(parser)!: drop legacy syntax",
		"refactor: tidy up\n\nbody line here explaining why",
	}
	for _, msg := range cases {
		vs := Lint(Parse(msg), Options{})
		if len(vs) != 0 {
			t.Errorf("%q 应合规，却报 %v", msg, rules(vs))
		}
	}
}

func TestParse_Fields(t *testing.T) {
	c := Parse("feat(api)!: 加分页")
	if !c.Parsed || c.Type != "feat" || c.Scope != "api" || !c.Breaking || c.Subject != "加分页" {
		t.Errorf("解析有误：%+v", c)
	}
}

func TestLint_TypeEnum(t *testing.T) {
	vs := Lint(Parse("frob: do a thing"), Options{})
	if !has(vs, "type-enum") {
		t.Errorf("frob 不在白名单应报 type-enum，got %v", rules(vs))
	}
}

func TestLint_TypeCase(t *testing.T) {
	vs := Lint(Parse("Feat: add x"), Options{})
	if !has(vs, "type-case") {
		t.Errorf("大写 type 应报 type-case，got %v", rules(vs))
	}
	if has(vs, "type-enum") {
		t.Error("Feat 小写后在白名单内，不该再报 type-enum")
	}
}

func TestLint_ScopeCase(t *testing.T) {
	vs := Lint(Parse("fix(API): x"), Options{})
	if !has(vs, "scope-case") {
		t.Errorf("大写 scope 应报 scope-case，got %v", rules(vs))
	}
}

func TestLint_Structure(t *testing.T) {
	vs := Lint(Parse("updated the readme"), Options{})
	if !has(vs, "header-structure") {
		t.Errorf("无 type 冒号结构应报 header-structure，got %v", rules(vs))
	}
}

func TestLint_SubjectEmptyAndFullStop(t *testing.T) {
	if vs := Lint(Parse("feat: "), Options{}); !has(vs, "subject-empty") {
		t.Errorf("空 subject 应报 subject-empty，got %v", rules(vs))
	}
	if vs := Lint(Parse("feat: add a thing."), Options{}); !has(vs, "subject-full-stop") {
		t.Errorf("结尾句号应报 subject-full-stop，got %v", rules(vs))
	}
}

func TestLint_HeaderMaxLength(t *testing.T) {
	long := "feat: " + strings.Repeat("x", 200)
	if vs := Lint(Parse(long), Options{}); !has(vs, "header-max-length") {
		t.Errorf("超长 header 应报 header-max-length，got %v", rules(vs))
	}
	// 中文按 rune 计：50 个汉字 + "feat: " = 56 rune，不超 100
	cn := "feat: " + strings.Repeat("码", 50)
	if vs := Lint(Parse(cn), Options{}); has(vs, "header-max-length") {
		t.Error("56 个 rune 不该超 100（不能按字节算）")
	}
}

func TestLint_BodyLeadingBlank(t *testing.T) {
	// header 紧跟 body、中间没空行
	vs := Lint(Parse("feat: x\nbody right after"), Options{})
	if !has(vs, "body-leading-blank") {
		t.Errorf("body 前缺空行应报 body-leading-blank，got %v", rules(vs))
	}
}

func TestLint_ScopeRequired(t *testing.T) {
	if vs := Lint(Parse("feat: x"), Options{ScopeRequired: true}); !has(vs, "scope-empty") {
		t.Errorf("--scope-required 下缺 scope 应报 scope-empty，got %v", rules(vs))
	}
	if vs := Lint(Parse("feat(api): x"), Options{ScopeRequired: true}); has(vs, "scope-empty") {
		t.Error("有 scope 时不该报 scope-empty")
	}
}

func TestLint_CustomTypes(t *testing.T) {
	// 覆盖白名单后 feat 反而不合法
	vs := Lint(Parse("feat: x"), Options{Types: []string{"task", "bug"}})
	if !has(vs, "type-enum") {
		t.Errorf("自定义白名单不含 feat，应报 type-enum，got %v", rules(vs))
	}
}

func TestParse_StripsCommentsAndScissors(t *testing.T) {
	raw := "feat: real subject\n" +
		"# 请输入提交说明。以 '#' 开头的行将被忽略\n" +
		"# ------------------------ >8 ------------------------\n" +
		"diff --git a/x b/x\n+noise\n"
	c := Parse(raw)
	if c.Header != "feat: real subject" {
		t.Errorf("应剥掉注释/scissors/diff，header=%q", c.Header)
	}
	if vs := Lint(c, Options{}); len(vs) != 0 {
		t.Errorf("清理后应合规，got %v", rules(vs))
	}
}

func TestLint_BreakingChangeFooter(t *testing.T) {
	c := Parse("feat: x\n\nbody\n\nBREAKING CHANGE: 删了旧 API")
	if !c.Breaking {
		t.Error("BREAKING CHANGE footer 应标记 Breaking")
	}
	if vs := Lint(c, Options{}); len(vs) != 0 {
		t.Errorf("带 BREAKING CHANGE 仍应合规，got %v", rules(vs))
	}
}
