package cli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestRoot 造一个和真 rootCmd 形状相同的树，用来逼 cobra 产生各种
// 「命令行写错了」的错误。
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "jdan", SilenceErrors: true, SilenceUsage: true}
	sub := &cobra.Command{
		Use:           "size",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE:          func(*cobra.Command, []string) error { return nil },
	}
	sub.Flags().Int("depth", 1, "")
	root.AddCommand(sub)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root
}

// errFrom 跑一遍 args，返回 cobra 实际产生的错误。
func errFrom(t *testing.T, args ...string) error {
	t.Helper()
	root := newTestRoot()
	root.SetArgs(args)
	_, err := root.ExecuteC()
	if err == nil {
		t.Fatalf("args %v 应该出错但没有", args)
	}
	return err
}

// isUsageError 靠匹配 cobra 的错误消息前缀工作 —— cobra 没导出哨兵值。
// 这里用**真实跑出来的** cobra 错误做断言，而不是手写字符串：cobra 升级
// 改了措辞，这条测试会立刻失败，而不是等到线上少了 --help 提示才发现。
func TestIsUsageError_ClassifiesRealCobraErrors(t *testing.T) {
	usage := map[string][]string{
		"未知命令":        {"sizz"},
		"未知 flag":     {"size", "--bogus"},
		"未知短 flag":    {"size", "-Z"},
		"flag 缺参数":    {"size", "--depth"},
		"flag 参数类型不对": {"size", "--depth", "abc"},
		"参数个数超限":      {"size", "a", "b", "c"},
	}
	for name, args := range usage {
		t.Run(name, func(t *testing.T) {
			err := errFrom(t, args...)
			if !isUsageError(err) {
				t.Errorf("应判为「命令行写错」并提示 --help，但没有\n  args: %v\n  err:  %q",
					args, err.Error())
			}
		})
	}
}

// 运行时错误不该提示 --help —— 文件不存在时叫用户去看 --help 是噪音。
func TestIsUsageError_RuntimeErrorsAreNotUsageErrors(t *testing.T) {
	cases := map[string]error{
		"文件不存在": errors.New(`无法读取 "/nope"：lstat /nope: no such file or directory`),
		"非目录":   errors.New(`"/etc/hosts" 不是目录`),
		"网络超时":  errors.New("dial tcp 1.2.3.4:443: i/o timeout"),
		"权限不足":  errors.New("open /private/var/db: permission denied"),
		"空错误":   errors.New(""),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if isUsageError(err) {
				t.Errorf("运行时错误不该提示 --help：%q", err.Error())
			}
		})
	}
}

// 未知命令的错误里必须仍然带着 cobra 的拼写建议 —— 这是这条路径上最有用的
// 信息，改动打印方式时不能把它弄丢。
func TestUnknownCommandKeepsSuggestion(t *testing.T) {
	err := errFrom(t, "sizz")
	msg := err.Error()
	if !strings.Contains(msg, "Did you mean this?") {
		t.Errorf("应保留 cobra 的拼写建议：%q", msg)
	}
	if !strings.Contains(msg, "size") {
		t.Errorf("建议里应出现 size：%q", msg)
	}
}

// 每个前缀都必须真能被某个 cobra 错误命中。挂了说明这条前缀已经过时
// （cobra 改了措辞或删了这类错误），留着是死代码。
func TestUsageErrorPrefixes_AllStillReachable(t *testing.T) {
	samples := [][]string{
		{"sizz"},
		{"size", "--bogus"},
		{"size", "-Z"},
		{"size", "--depth"},
		{"size", "--depth", "abc"},
		{"size", "a", "b", "c"},
	}
	hit := make(map[string]bool)
	for _, args := range samples {
		root := newTestRoot()
		root.SetArgs(args)
		if _, err := root.ExecuteC(); err != nil {
			for _, p := range usageErrorPrefixes {
				if strings.HasPrefix(err.Error(), p) {
					hit[p] = true
				}
			}
		}
	}
	// "requires at least" 和 "unknown help topic" 本命令树里造不出来，
	// 单独说明而不是硬凑样本。
	optional := map[string]bool{"requires at least": true, "unknown help topic": true}
	for _, p := range usageErrorPrefixes {
		if !hit[p] && !optional[p] {
			t.Errorf("前缀 %q 已无对应的 cobra 错误，可能过时了", p)
		}
	}
}
