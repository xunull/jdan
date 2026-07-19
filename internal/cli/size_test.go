package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/sizex"
)

func sizeTree(t *testing.T, spec map[string]int) string {
	t.Helper()
	root := t.TempDir()
	for rel, size := range spec {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runSize(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	cmd := newSizeCommand(sizeCmdDeps{out: &out, errOut: &errOut})
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestSizeCommand_RanksChildrenBySize(t *testing.T) {
	root := sizeTree(t, map[string]int{
		"big/f": 400000, "mid/f": 200000, "small/f": 50000,
	})
	out, _, err := runSize(t, root)
	if err != nil {
		t.Fatal(err)
	}
	bigAt, midAt, smallAt := strings.Index(out, "big"), strings.Index(out, "mid"), strings.Index(out, "small")
	if bigAt < 0 || midAt < 0 || smallAt < 0 {
		t.Fatalf("输出缺少子项：\n%s", out)
	}
	if !(bigAt < midAt && midAt < smallAt) {
		t.Errorf("应按体积降序排列：\n%s", out)
	}
	if !strings.Contains(out, "%") {
		t.Errorf("子项应带百分比：\n%s", out)
	}
}

func TestSizeCommand_JSONIsFullTreeRegardlessOfTopAndDepth(t *testing.T) {
	root := sizeTree(t, map[string]int{
		"a/deep/deeper/f": 100000, "b/f": 100000, "c/f": 100000, "d/f": 100000, "e/f": 100000,
	})
	out, _, err := runSize(t, root, "--json", "--top", "1", "--depth", "1")
	if err != nil {
		t.Fatal(err)
	}
	var got sizex.JSONRoot
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON 不可解析：%v\n%s", err, out)
	}
	if len(got.Children) != 5 {
		t.Errorf("--json 应无视 --top 输出全部 5 个子项，得到 %d", len(got.Children))
	}
	// --depth 1 也不该裁剪 JSON 的深度
	var depth func(*sizex.JSONNode) int
	depth = func(n *sizex.JSONNode) int {
		d := 0
		for _, c := range n.Children {
			d = max(d, depth(c))
		}
		return d + 1
	}
	if d := depth(got.JSONNode); d < 4 {
		t.Errorf("--json 应无视 --depth 输出全树（a/deep/deeper 共 4 层），得到 %d 层", d)
	}
}

func TestSizeCommand_JSONByteIdenticalAcrossRuns(t *testing.T) {
	spec := map[string]int{}
	for _, n := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
		spec[n+"/f"] = 100000 // 同体积，最大化排序歧义
		spec[n+"/sub/f"] = 50000
	}
	root := sizeTree(t, spec)

	var first string
	for i := range 3 {
		out, _, err := runSize(t, root, "--json", "--jobs", "8")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = out
			continue
		}
		if out != first {
			t.Fatalf("第 %d 次 --json 输出与第 1 次不同", i)
		}
	}
}

func TestSizeCommand_HiddenEntriesNeedAllFlag(t *testing.T) {
	root := sizeTree(t, map[string]int{"visible/f": 100000, ".hidden/f": 100000})

	def, _, err := runSize(t, root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	all, _, err := runSize(t, root, "--json", "--all")
	if err != nil {
		t.Fatal(err)
	}
	var d, a sizex.JSONRoot
	if err := json.Unmarshal([]byte(def), &d); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(all), &a); err != nil {
		t.Fatal(err)
	}
	if a.Bytes <= d.Bytes {
		t.Errorf("--all 应计入隐藏项：默认 %d，--all %d", d.Bytes, a.Bytes)
	}
}

// --files 是回答「哪一个文件吃了空间」的唯一途径，默认只列目录时看不到。
func TestSizeCommand_FilesFlagListsFiles(t *testing.T) {
	root := sizeTree(t, map[string]int{"huge.bin": 500000, "sub/small": 1000})

	def, _, err := runSize(t, root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	withFiles, _, err := runSize(t, root, "--json", "--files")
	if err != nil {
		t.Fatal(err)
	}

	var d, f sizex.JSONRoot
	if err := json.Unmarshal([]byte(def), &d); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(withFiles), &f); err != nil {
		t.Fatal(err)
	}

	// 关键：总量必须一致，--files 只改变呈现粒度，不改变统计
	if d.Bytes != f.Bytes {
		t.Errorf("--files 不应改变总量：默认 %d vs --files %d（双重计数？）", d.Bytes, f.Bytes)
	}
	if d.Files != f.Files {
		t.Errorf("--files 不应改变文件计数：默认 %d vs --files %d", d.Files, f.Files)
	}

	var sawFile bool
	for _, c := range f.Children {
		if c.Type == "file" && strings.HasSuffix(c.Path, "huge.bin") {
			sawFile = true
		}
	}
	if !sawFile {
		t.Errorf("--files 应把 huge.bin 作为 type=file 节点列出：%s", withFiles)
	}
	for _, c := range d.Children {
		if c.Type == "file" {
			t.Errorf("默认不应有 file 节点：%s", def)
		}
	}
}

func TestSizeCommand_ApparentDiffersFromActual(t *testing.T) {
	if !sizex.Supported {
		t.Skip("本平台无 st_blocks")
	}
	spec := map[string]int{}
	for i := range 100 {
		spec["f"+string(rune('a'+i%26))+string(rune('a'+i/26))] = 1 // 100 个 1 字节文件
	}
	root := sizeTree(t, spec)

	actual, _, err := runSize(t, root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	apparent, _, err := runSize(t, root, "--json", "--apparent")
	if err != nil {
		t.Fatal(err)
	}
	var ac, ap sizex.JSONRoot
	if err := json.Unmarshal([]byte(actual), &ac); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(apparent), &ap); err != nil {
		t.Fatal(err)
	}
	if ap.Bytes >= ac.Bytes {
		t.Errorf("大量小文件时逻辑大小 %d 应远小于实际占盘 %d（块取整）", ap.Bytes, ac.Bytes)
	}
	if !ap.Apparent {
		t.Error("--apparent 应在 JSON 里标记")
	}
	// 文本模式要有头部提示，否则用户不知道数字换了含义
	txt, _, err := runSize(t, root, "--apparent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, "--apparent") {
		t.Errorf("--apparent 文本输出应有头部提示：\n%s", txt)
	}
}

func TestSizeCommand_TopAggregatesRemainder(t *testing.T) {
	spec := map[string]int{}
	for i, n := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		spec[n+"/f"] = (10 - i) * 100000
	}
	root := sizeTree(t, spec)

	out, _, err := runSize(t, root, "--top", "3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "其他 4 项") {
		t.Errorf("--top 3 应把剩余 4 项聚合：\n%s", out)
	}
	// 聚合行必须在最后一行数据
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	var lastData string
	for _, ln := range lines {
		if strings.Contains(ln, "%") {
			lastData = ln
		}
	}
	if !strings.Contains(lastData, "其他") {
		t.Errorf("聚合行应在末尾，实际末行是：%q", lastData)
	}
}

func TestSizeCommand_ErrorsOnBadPath(t *testing.T) {
	if _, _, err := runSize(t, filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("路径不存在应返回错误")
	}
	root := sizeTree(t, map[string]int{"f": 100})
	if _, _, err := runSize(t, filepath.Join(root, "f")); err == nil {
		t.Error("参数是文件应返回错误")
	}
}

// 权限错误不应让命令失败，但必须在页脚告知。
func TestSizeCommand_PermissionErrorsWarnButSucceed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 不受权限限制")
	}
	root := sizeTree(t, map[string]int{"ok/f": 100000, "denied/f": 100000})
	denied := filepath.Join(root, "denied")
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { os.Chmod(denied, 0o755) })

	out, _, err := runSize(t, root)
	if err != nil {
		t.Fatalf("权限错误不应让命令失败：%v", err)
	}
	if !strings.Contains(out, "无权访问") {
		t.Errorf("页脚应告知无权访问：\n%s", out)
	}
	if !strings.Contains(out, "--verbose") {
		t.Errorf("应提示 --verbose：\n%s", out)
	}

	v, _, err := runSize(t, root, "--verbose")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v, "denied") {
		t.Errorf("--verbose 应列出具体路径：\n%s", v)
	}
}

// 非 TTY 时进度必须静默，否则会污染管道下游。
func TestSizeCommand_NoProgressOnNonTTY(t *testing.T) {
	root := sizeTree(t, map[string]int{"a/f": 100000})
	_, stderr, err := runSize(t, root)
	if err != nil {
		t.Fatal(err)
	}
	if stderr != "" {
		t.Errorf("非 TTY 时 stderr 应为空，得到 %q", stderr)
	}
}

// --jobs 不影响结果，只影响速度。
func TestSizeCommand_JobsDoesNotChangeResult(t *testing.T) {
	root := sizeTree(t, map[string]int{
		"a/b/c/f": 100000, "d/e/f": 200000, "g/f": 300000,
	})
	var want string
	for _, jobs := range []string{"1", "2", "8", "16"} {
		out, _, err := runSize(t, root, "--json", "--jobs", jobs)
		if err != nil {
			t.Fatalf("jobs=%s: %v", jobs, err)
		}
		if want == "" {
			want = out
			continue
		}
		if out != want {
			t.Errorf("jobs=%s 的输出与 jobs=1 不同", jobs)
		}
	}
}

func TestSizeCommand_NoColorFlagStripsANSI(t *testing.T) {
	root := sizeTree(t, map[string]int{"a/f": 100000})
	out, _, err := runSize(t, root, "--no-color")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b") {
		t.Errorf("--no-color 不应有 ANSI 转义：%q", out)
	}
}

func TestDefaultSizeJobs_IsSane(t *testing.T) {
	n := defaultSizeJobs()
	if n < 1 || n > 16 {
		t.Errorf("默认并发度 %d 应在 1..16 之间", n)
	}
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if !noColorEnv() {
		t.Error("设了 NO_COLOR（即使为空）就应关闭颜色，这是 no-color.org 的约定")
	}
	os.Unsetenv("NO_COLOR")
	if noColorEnv() {
		t.Error("未设 NO_COLOR 时不应关闭颜色")
	}
}
