package sizex

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/xunull/jdan/internal/termx"
)

// 同层必须按 (Bytes 降序, Name 升序)。
func TestBuildTree_SortsBySizeThenName(t *testing.T) {
	root := mktree(t, map[string]int{
		"big/f": 300000, "mid/f": 200000, "small/f": 100000,
	})
	tr := BuildTree(scan(t, root, nil), TreeOptions{Depth: 1})

	var names []string
	for _, k := range tr.Kids {
		names = append(names, k.Name)
	}
	want := []string{"big", "mid", "small"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("排序应为 %v，得到 %v", want, names)
	}
}

// 体积相同时靠 Name 兜底消歧。没有这条，顺序就取决于 map 遍历，
// 「连跑 3 次逐字节相同」的验收会挂。
func TestBuildTree_TiesBrokenByNameDeterministically(t *testing.T) {
	spec := map[string]int{}
	for _, n := range []string{"delta", "alpha", "charlie", "bravo", "echo"} {
		spec[n+"/f"] = 100000 // 全部同样大小
	}
	root := mktree(t, spec)
	res := scan(t, root, nil)

	var first []string
	for i := range 20 {
		var names []string
		for _, k := range BuildTree(res, TreeOptions{Depth: 1}).Kids {
			names = append(names, k.Name)
		}
		if i == 0 {
			first = names
			continue
		}
		if strings.Join(names, ",") != strings.Join(first, ",") {
			t.Fatalf("第 %d 次拼树顺序变了：%v vs %v", i, first, names)
		}
	}
	want := "alpha,bravo,charlie,delta,echo"
	if got := strings.Join(first, ","); got != want {
		t.Errorf("同体积应按 Name 升序：想要 %s，得到 %s", want, got)
	}
}

// --top 截断后剩余项合并为聚合行，且聚合行永远在末尾。
func TestBuildTree_TopAggregatesRemainderAtEnd(t *testing.T) {
	spec := map[string]int{}
	for i, n := range []string{"a", "b", "c", "d", "e", "f"} {
		spec[n+"/f"] = (10 - i) * 100000 // a 最大，f 最小
	}
	root := mktree(t, spec)
	tr := BuildTree(scan(t, root, nil), TreeOptions{Depth: 1, Top: 3})

	if len(tr.Kids) != 4 {
		t.Fatalf("top=3 应得 3 项 + 1 聚合行，得到 %d 项", len(tr.Kids))
	}
	last := tr.Kids[3]
	if last.Aggregated != 3 {
		t.Errorf("聚合行应代表 3 项，得到 %d", last.Aggregated)
	}
	if !strings.Contains(last.displayName(), "其他 3 项") {
		t.Errorf("聚合行显示名 = %q", last.displayName())
	}
	for i, k := range tr.Kids[:3] {
		if k.Aggregated != 0 {
			t.Errorf("第 %d 项不该是聚合行", i)
		}
	}
}

// 关键：聚合行的总和大于榜上最后一名时，它仍然钉在末尾，不参与排序。
// 长尾场景（--top 10 配 50 个子目录）下这是常态。
func TestBuildTree_AggregateRowStaysLastEvenWhenLarger(t *testing.T) {
	spec := map[string]int{
		"aa/f": 900000, // 第 1
		"bb/f": 800000, // 第 2
	}
	// 后面 8 个各 500000，加起来 4000000 远大于第 2 名
	for _, n := range []string{"c", "d", "e", "f", "g", "h", "i", "j"} {
		spec[n+"/f"] = 500000
	}
	root := mktree(t, spec)
	tr := BuildTree(scan(t, root, nil), TreeOptions{Depth: 1, Top: 2})

	last := tr.Kids[len(tr.Kids)-1]
	if last.Aggregated == 0 {
		t.Fatal("最后一项应是聚合行")
	}
	if last.Bytes <= tr.Kids[1].Bytes {
		t.Skip("聚合行未大于第 2 名，本用例前提不成立")
	}
	t.Logf("聚合行 %d 字节 > 第 2 名 %d 字节，仍在末尾 ✓", last.Bytes, tr.Kids[1].Bytes)
}

// --top 边界处体积相同时，谁进榜由 Name 决定，不随运行变化。
func TestBuildTree_TopBoundaryTieIsDeterministic(t *testing.T) {
	spec := map[string]int{"big/f": 900000}
	for _, n := range []string{"m", "n", "o", "p"} {
		spec[n+"/f"] = 100000 // 四个并列，top=2 时要切在它们中间
	}
	root := mktree(t, spec)
	res := scan(t, root, nil)

	var first string
	for i := range 20 {
		tr := BuildTree(res, TreeOptions{Depth: 1, Top: 2})
		var names []string
		for _, k := range tr.Kids {
			names = append(names, k.displayName())
		}
		got := strings.Join(names, ",")
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("第 %d 次 top 截断边界变了：%s vs %s", i, first, got)
		}
	}
	if !strings.Contains(first, "big") || !strings.Contains(first, "m") {
		t.Errorf("并列时应按 Name 取 m 进榜：%s", first)
	}
}

// --depth 控制展开层数，不影响体积统计。
func TestBuildTree_DepthLimitsExpansionNotTotals(t *testing.T) {
	root := mktree(t, map[string]int{"a/b/c/f": 400000, "a/b/g": 100000})
	res := scan(t, root, nil)

	d1 := BuildTree(res, TreeOptions{Depth: 1})
	d3 := BuildTree(res, TreeOptions{Depth: 3})

	if d1.Bytes != d3.Bytes {
		t.Errorf("--depth 不应改变总量：%d vs %d", d1.Bytes, d3.Bytes)
	}
	if len(d1.Kids) != 1 || len(d1.Kids[0].Kids) != 0 {
		t.Errorf("depth=1 应只展开一层，得到 %d 层", depthOf(d1))
	}
	if depthOf(d3) < 3 {
		t.Errorf("depth=3 应展开三层，得到 %d 层", depthOf(d3))
	}
	// 未展开的层，其体积仍要算在祖先里
	if d1.Kids[0].Bytes != d3.Kids[0].Bytes {
		t.Errorf("未展开子项的体积应仍计入：%d vs %d", d1.Kids[0].Bytes, d3.Kids[0].Bytes)
	}
}

func depthOf(t *Tree) int {
	deepest := 0
	for _, k := range t.Kids {
		deepest = max(deepest, depthOf(k))
	}
	return deepest + 1
}

// 错误列表必须按路径排序 —— 并发时是各 worker 往共享 slice 里 append 的。
func TestSortedErrors_IsOrderedByPath(t *testing.T) {
	r := &Result{Errors: []ScanError{
		{Path: "/z", Err: errTest}, {Path: "/a", Err: errTest}, {Path: "/m", Err: errTest},
	}}
	got := r.SortedErrors()
	if got[0].Path != "/a" || got[1].Path != "/m" || got[2].Path != "/z" {
		t.Errorf("应按路径升序，得到 %v", got)
	}
	// 不能改动原 slice
	if r.Errors[0].Path != "/z" {
		t.Error("SortedErrors 不应原地修改 Result.Errors")
	}
}

var errTest = errTestType{}

type errTestType struct{}

func (errTestType) Error() string { return "test" }

// ---- JSON ----

// --json 永远输出全树，不受 --top / --depth 影响。
func TestJSONData_IgnoresDepthAndTop(t *testing.T) {
	root := mktree(t, map[string]int{
		"a/b/c/f": 100000, "d/f": 100000, "e/f": 100000, "g/f": 100000,
	})
	res := scan(t, root, nil)
	data := res.JSONData()

	if len(data.Children) != 4 {
		t.Errorf("JSON 应含全部 4 个顶层子项，得到 %d", len(data.Children))
	}
	// 深层必须还在
	var deepest func(*JSONNode) int
	deepest = func(n *JSONNode) int {
		d := 0
		for _, c := range n.Children {
			d = max(d, deepest(c))
		}
		return d + 1
	}
	if got := deepest(data.JSONNode); got < 4 {
		t.Errorf("JSON 应输出全树（a/b/c 共 4 层），得到 %d 层", got)
	}
}

// 连跑 3 次 --json 必须逐字节相同。
func TestJSONData_ByteIdenticalAcrossRuns(t *testing.T) {
	spec := map[string]int{}
	for _, n := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot"} {
		spec[n+"/f"] = 100000 // 全部同体积，最大化排序歧义
		spec[n+"/sub/f"] = 50000
	}
	root := mktree(t, spec)

	var first []byte
	for i := range 3 {
		res := scan(t, root, nil)
		b, err := json.Marshal(res.JSONData())
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = b
			continue
		}
		if string(b) != string(first) {
			t.Fatalf("第 %d 次 JSON 与第 1 次不同", i)
		}
	}
}

func TestJSONData_HasTypeAndRootOnlyFlags(t *testing.T) {
	root := mktree(t, map[string]int{"a/f": 100000})
	res := scan(t, root, func(o *Options) { o.Apparent = true })

	b, err := json.Marshal(res.JSONData())
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"type":"dir"`) {
		t.Errorf("节点应带 type 字段：%s", s)
	}
	if !strings.Contains(s, `"apparent":true`) {
		t.Errorf("根应带 apparent 标记：%s", s)
	}
	// apparent / supported 只在根出现一次
	if strings.Count(s, `"apparent"`) != 1 {
		t.Errorf("apparent 只应在根出现一次，出现了 %d 次", strings.Count(s, `"apparent"`))
	}
	if strings.Count(s, `"supported"`) != 1 {
		t.Errorf("supported 只应在根出现一次，出现了 %d 次", strings.Count(s, `"supported"`))
	}
}

// ---- 渲染 ----

// 根行不带条形图和百分比：根的分母（父目录或磁盘总量）不在扫描范围内，
// 硬给一个百分比是骗人。
func TestRender_RootRowHasNoPercentage(t *testing.T) {
	root := mktree(t, map[string]int{"a/f": 300000, "b/f": 100000})
	res := scan(t, root, nil)
	out := Render(res, BuildTree(res, TreeOptions{Depth: 1}), RenderOptions{})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if strings.Contains(lines[0], "%") {
		t.Errorf("根行不应有百分比：%q", lines[0])
	}
	if strings.Contains(lines[0], "█") || strings.Contains(lines[0], "░") {
		t.Errorf("根行不应有条形图：%q", lines[0])
	}
	if !strings.Contains(out, "%") {
		t.Error("子项应有百分比")
	}
}

// 同层百分比之和应为 100%（分母是直接父级）。
func TestRender_SiblingPercentagesSumTo100(t *testing.T) {
	root := mktree(t, map[string]int{"a/f": 300000, "b/f": 200000, "c/f": 100000})
	res := scan(t, root, nil)
	tr := BuildTree(res, TreeOptions{Depth: 1})

	var sum float64
	for _, k := range tr.Kids {
		sum += percentOf(k.Bytes, tr.Bytes)
	}
	if sum < 99.0 || sum > 100.01 {
		t.Errorf("同层百分比之和 = %.2f%%，应接近 100%%（差额是根目录自身占盘）", sum)
	}
}

func TestRender_FooterReportsErrorsAndDedup(t *testing.T) {
	res := &Result{
		Root:      "/x",
		Nodes:     map[string]*Node{"/x": {Path: "/x", Bytes: 1024}},
		Supported: true,
		Deduped:   1234,
		Errors:    []ScanError{{Path: "/x/denied", Err: errTest}},
	}
	tr := BuildTree(res, TreeOptions{Depth: 1})

	out := Render(res, tr, RenderOptions{Elapsed: 2300 * time.Millisecond})
	for _, want := range []string{"用时 2.3s", "1 个目录无权访问", "--verbose", "1,234 个硬链接已去重"} {
		if !strings.Contains(out, want) {
			t.Errorf("页脚缺 %q：\n%s", want, out)
		}
	}
	// --verbose 列出具体路径，且不再提示 --verbose
	v := Render(res, tr, RenderOptions{Verbose: true})
	if !strings.Contains(v, "/x/denied") {
		t.Errorf("--verbose 应列出路径：\n%s", v)
	}
	if strings.Contains(v, "--verbose 看详情") {
		t.Errorf("--verbose 模式不该再提示 --verbose：\n%s", v)
	}
}

// 数字含义与默认不同时必须在头部说清楚。
func TestRender_HeaderNoteWhenApparentOrUnsupported(t *testing.T) {
	base := map[string]*Node{"/x": {Path: "/x", Bytes: 1024}}

	ap := &Result{Root: "/x", Nodes: base, Supported: true, Apparent: true}
	if out := Render(ap, BuildTree(ap, TreeOptions{}), RenderOptions{}); !strings.Contains(out, "--apparent") {
		t.Errorf("apparent 模式应有头部提示：\n%s", out)
	}

	un := &Result{Root: "/x", Nodes: base, Supported: false}
	if out := Render(un, BuildTree(un, TreeOptions{}), RenderOptions{}); !strings.Contains(out, "无法测量实际占盘") {
		t.Errorf("不支持的平台应有头部提示：\n%s", out)
	}

	ok := &Result{Root: "/x", Nodes: base, Supported: true}
	if out := Render(ok, BuildTree(ok, TreeOptions{}), RenderOptions{}); strings.Contains(out, "注：") {
		t.Errorf("正常模式不该有头部提示：\n%s", out)
	}
}

// 窄终端下每行可见宽度不超限。
func TestRender_TruncatesToMaxWidth(t *testing.T) {
	root := mktree(t, map[string]int{
		"这是一个名字非常长的目录用来测试中间省略号截断行为是否正确/f":                             300000,
		"another-extremely-long-directory-name-for-truncation-test/f": 100000,
	})
	res := scan(t, root, nil)
	out := Render(res, BuildTree(res, TreeOptions{Depth: 1}), RenderOptions{MaxWidth: 60})

	for ln := range strings.SplitSeq(strings.TrimRight(out, "\n"), "\n") {
		if strings.HasPrefix(ln, "/") || ln == "" {
			continue // 根行是完整路径，不截断
		}
		if w := lineWidth(ln); w > 60 {
			t.Errorf("行可见宽度 %d > 60：%q", w, ln)
		}
	}
}

func lineWidth(s string) int { return termx.VisWidth(s) }
