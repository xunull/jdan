package cli

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
)

// 钉死：注册补全的命令路径必须仍然存在（以后改命令名会让这里失败，而不是静默丢补全）。
func TestCompletionPathsResolve(t *testing.T) {
	paths := [][]string{
		{"hash"}, {"totp"}, {"qr"}, {"figlet"}, {"cert"}, {"ascii-art"},
		{"ssl", "pin"}, {"http", "headers"}, {"net", "probe"}, {"json", "merge"},
		{"dns", "lookup"}, {"dns", "trace"}, {"dns", "reverse"}, {"ping"}, {"cal"},
	}
	for _, p := range paths {
		c, _, err := rootCmd.Find(p)
		if err != nil || c == nil {
			t.Errorf("命令路径 %v 解析失败：%v", p, err)
		}
	}
}

func TestRegisterCompletions_NoPanic(t *testing.T) {
	registerCompletions() // 重复调用也不应 panic
	registerCompletions()
}

func TestFlagCompletion_Values(t *testing.T) {
	registerCompletions()
	cases := []struct {
		path []string
		flag string
		want string
	}{
		{[]string{"ascii-art"}, "ramp", "blocks"},
		{[]string{"hash"}, "algo", "sha256"},
		{[]string{"qr"}, "ecc", "H"},
		{[]string{"ssl", "pin"}, "format", "okhttp"},
		{[]string{"dns", "lookup"}, "type", "AAAA"},
		{[]string{"json", "merge"}, "arrays", "append"},
		{[]string{"cert"}, "key-type", "ed25519"},
	}
	for _, c := range cases {
		cmd, _, err := rootCmd.Find(c.path)
		if err != nil || cmd == nil {
			t.Errorf("%v 解析失败", c.path)
			continue
		}
		f, ok := cmd.GetFlagCompletionFunc(c.flag)
		if !ok {
			t.Errorf("%v --%s 未注册补全", c.path, c.flag)
			continue
		}
		vals, _ := f(cmd, nil, "")
		if !slices.Contains(vals, c.want) {
			t.Errorf("%v --%s 补全应含 %q，got %v", c.path, c.flag, c.want, vals)
		}
	}
}

func TestFlagCompletion_DoHDynamic(t *testing.T) {
	registerCompletions()
	cmd, _, _ := rootCmd.Find([]string{"ping"})
	f, ok := cmd.GetFlagCompletionFunc("doh")
	if !ok {
		t.Fatal("ping --doh 未注册补全")
	}
	vals, _ := f(cmd, nil, "")
	if len(vals) == 0 {
		t.Error("--doh 应补出 provider 别名（动态）")
	}
}

func TestMonthComplete(t *testing.T) {
	got, dir := monthComplete(nil, nil, "")
	if !slices.Contains(got, "Jan") || !slices.Contains(got, "12") {
		t.Errorf("月份补全应含 1..12 + Jan…，got %v", got)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Error("应 NoFileComp")
	}
	// 已有第一个参数（年份位）后不再补月份
	if got2, _ := monthComplete(nil, []string{"6"}, ""); got2 != nil {
		t.Errorf("有第一参数后不应再补，got %v", got2)
	}
}
