package sizex

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mktree 造一棵目录树。spec 的键是相对路径，值是文件字节数；
// 以 / 结尾的键建成空目录。
func mktree(t *testing.T, spec map[string]int) string {
	t.Helper()
	root := t.TempDir()
	for rel, size := range spec {
		p := filepath.Join(root, rel)
		if strings.HasSuffix(rel, "/") {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scan(t *testing.T, root string, tweak func(*Options)) *Result {
	t.Helper()
	opts := Options{Root: root, OneFileSystem: true}
	if tweak != nil {
		tweak(&opts)
	}
	res, err := Scan(opts)
	if err != nil {
		t.Fatalf("Scan(%s): %v", root, err)
	}
	return res
}

// at 按相对路径取节点。必须从 res.Root 而非原始 root 拼：Scan 会解析根参数的
// 符号链接（macOS 的 t.TempDir() 给的是 /var/… → /private/var/…），Nodes 的键
// 是解析后的路径。
func at(t *testing.T, res *Result, rel string) *Node {
	t.Helper()
	p := filepath.Join(res.Root, rel)
	n, ok := res.Nodes[p]
	if !ok {
		t.Fatalf("节点 %s 不存在（Root=%s）", p, res.Root)
	}
	return n
}

func has(res *Result, rel string) bool {
	_, ok := res.Nodes[filepath.Join(res.Root, rel)]
	return ok
}

// duKiB 返回 du -sk 报告的 KiB 数。
func duKiB(t *testing.T, path string, args ...string) uint64 {
	t.Helper()
	if _, err := exec.LookPath("du"); err != nil {
		t.Skip("找不到 du")
	}
	out, err := exec.Command("du", append(append([]string{"-sk"}, args...), path)...).Output()
	if err != nil {
		t.Skipf("du 执行失败：%v", err)
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		t.Skip("du 输出为空")
	}
	n, err := strconv.ParseUint(f[0], 10, 64)
	if err != nil {
		t.Skipf("解析 du 输出失败：%v", err)
	}
	return n
}

// 核心验收：根总量与 du -sk 一致。无硬链接的普通树。
func TestScan_RootTotalMatchesDu(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	root := mktree(t, map[string]int{
		"a/f1":       100000,
		"a/f2":       3,
		"a/b/f3":     50000,
		"a/b/c/f4":   1,
		"d/f5":       200000,
		"empty/":     0,
		"deep/x/y/z": 4096,
	})
	res := scan(t, root, nil)
	want := duKiB(t, root) * 1024
	if res.Total() != want {
		t.Errorf("根总量 %d 与 du -sk 的 %d 不符（差 %d 字节）",
			res.Total(), want, int64(res.Total())-int64(want))
	}
}

// 含硬链接的树，根总量仍须与 du 一致 —— du 默认就去重。
func TestScan_RootTotalMatchesDu_WithHardlinks(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 inode 信息")
	}
	root := mktree(t, map[string]int{
		"aaa/orig":  100000,
		"zzz/other": 8192,
	})
	orig := filepath.Join(root, "aaa", "orig")
	for _, link := range []string{"zzz/link1", "aaa/link2"} {
		if err := os.Link(orig, filepath.Join(root, link)); err != nil {
			t.Skipf("不支持硬链接：%v", err)
		}
	}

	res := scan(t, root, nil)
	if res.Deduped != 2 {
		t.Errorf("应去重掉 2 个重复链接，得到 %d", res.Deduped)
	}
	want := duKiB(t, root) * 1024
	if res.Total() != want {
		t.Errorf("含硬链接时根总量 %d 与 du -sk 的 %d 不符", res.Total(), want)
	}
}

// 硬链接归属必须确定：同一棵树扫多次，每个目录的数字都不能变。
// 这是并发化的前提 —— du 的「先遇到的算」在并发下会让子树数字互换。
func TestScan_HardlinkAttributionIsDeterministic(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 inode 信息")
	}
	root := mktree(t, map[string]int{"aaa/orig": 65536, "mmm/x": 1, "zzz/y": 1})
	orig := filepath.Join(root, "aaa", "orig")
	for _, l := range []string{"zzz/link", "mmm/link"} {
		if err := os.Link(orig, filepath.Join(root, l)); err != nil {
			t.Skipf("不支持硬链接：%v", err)
		}
	}

	first := scan(t, root, nil)
	for i := range 5 {
		next := scan(t, root, nil)
		if len(first.Nodes) != len(next.Nodes) {
			t.Fatalf("第 %d 次扫描节点数不同", i)
		}
		for p, n := range first.Nodes {
			if next.Nodes[p].Bytes != n.Bytes {
				t.Errorf("第 %d 次扫描 %s 的 Bytes 变了：%d → %d",
					i, p, n.Bytes, next.Nodes[p].Bytes)
			}
		}
	}
}

// 归属给字典序最小的路径：aaa/orig < mmm/link < zzz/link，所以 64 KiB 记在 aaa 下。
func TestScan_HardlinkGoesToLexicographicallySmallestPath(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 inode 信息")
	}
	root := mktree(t, map[string]int{"aaa/orig": 65536, "mmm/pad": 1, "zzz/pad": 1})
	orig := filepath.Join(root, "aaa", "orig")
	for _, l := range []string{"zzz/link", "mmm/link"} {
		if err := os.Link(orig, filepath.Join(root, l)); err != nil {
			t.Skipf("不支持硬链接：%v", err)
		}
	}
	res := scan(t, root, nil)

	aaa := at(t, res, "aaa").Bytes
	mmm := at(t, res, "mmm").Bytes
	zzz := at(t, res, "zzz").Bytes
	if aaa <= mmm || aaa <= zzz {
		t.Errorf("64KiB 应记在字典序最小的 aaa 下：aaa=%d mmm=%d zzz=%d", aaa, mmm, zzz)
	}
}

// 每个目录的 Bytes 必须等于「自身 + 所有直属子目录的 Bytes」。
// 这条自洽性 du 本身也满足，去重不会破坏它。
func TestScan_ParentEqualsSelfPlusChildren(t *testing.T) {
	root := mktree(t, map[string]int{
		"a/f": 10000, "a/b/f": 20000, "a/b/c/f": 30000, "d/f": 40000,
	})
	res := scan(t, root, nil)

	children := map[string][]*Node{}
	for p, n := range res.Nodes {
		if p == res.Root {
			continue
		}
		parent := filepath.Dir(p)
		children[parent] = append(children[parent], n)
	}
	for p, n := range res.Nodes {
		var sum uint64
		for _, c := range children[p] {
			sum += c.Bytes
		}
		if n.Bytes != n.selfBytes+sum {
			t.Errorf("%s: Bytes=%d != selfBytes=%d + Σchildren=%d", p, n.Bytes, n.selfBytes, sum)
		}
	}
}

// 文件计数按物理出现次数算，与体积去重无关：3 条硬链接算 3 个文件。
func TestScan_FileCountCountsAllLinks(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 inode 信息")
	}
	root := mktree(t, map[string]int{"orig": 4096})
	for _, l := range []string{"l1", "l2"} {
		if err := os.Link(filepath.Join(root, "orig"), filepath.Join(root, l)); err != nil {
			t.Skipf("不支持硬链接：%v", err)
		}
	}
	res := scan(t, root, nil)
	if res.TotalFiles() != 3 {
		t.Errorf("文件计数应为 3（物理出现次数），得到 %d", res.TotalFiles())
	}
}

// 默认跳过隐藏条目，--all 计入。
func TestScan_HiddenEntries(t *testing.T) {
	root := mktree(t, map[string]int{"visible": 100000, ".hidden": 100000, ".hdir/f": 100000})

	def := scan(t, root, nil)
	all := scan(t, root, func(o *Options) { o.IncludeHidden = true })

	if all.Total() <= def.Total() {
		t.Errorf("--all 应计入更多：默认 %d，全部 %d", def.Total(), all.Total())
	}
	if has(def, ".hdir") {
		t.Error("默认不应为隐藏目录建节点")
	}
	if !has(all, ".hdir") {
		t.Error("--all 应为隐藏目录建节点")
	}
}

// 符号链接不跟随：链接自身只占几十字节，不该把目标的体积算进来。
func TestScan_DoesNotFollowSymlinks(t *testing.T) {
	root := mktree(t, map[string]int{"real/big": 1 << 20, "linkdir/pad": 1})
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "linkdir", "to-real")); err != nil {
		t.Skipf("不支持符号链接：%v", err)
	}
	res := scan(t, root, nil)

	linkdir := at(t, res, "linkdir").Bytes
	if linkdir >= 1<<20 {
		t.Errorf("linkdir 占盘 %d 不应包含符号链接目标的 1 MiB", linkdir)
	}
	if has(res, "linkdir/to-real") {
		t.Error("不应为符号链接建目录节点（那会重复计数并可能绕环）")
	}
}

// 权限错误收集但不中断，且退出时仍有可用结果。
func TestScan_PermissionErrorsAreCollectedNotFatal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 不受权限限制")
	}
	root := mktree(t, map[string]int{"ok/f": 100000, "denied/f": 100000})
	denied := filepath.Join(root, "denied")
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { os.Chmod(denied, 0o755) })

	res, err := Scan(Options{Root: root, OneFileSystem: true})
	if err != nil {
		t.Fatalf("权限错误不应让整次扫描失败：%v", err)
	}
	if len(res.Errors) == 0 {
		t.Error("应收集到至少一条权限错误")
	}
	if res.Total() == 0 {
		t.Error("即使有权限错误也应给出可用的部分结果")
	}
	if at(t, res, "ok").Bytes == 0 {
		t.Error("可访问的子树应正常统计")
	}
}

// apparent 模式量逻辑大小：大量小文件时应明显小于实际占盘（块取整）。
func TestScan_ApparentIsSmallerForManyTinyFiles(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	spec := map[string]int{}
	for i := range 200 {
		spec["f"+strconv.Itoa(i)] = 1
	}
	root := mktree(t, spec)

	actual := scan(t, root, nil)
	apparent := scan(t, root, func(o *Options) { o.Apparent = true })

	if apparent.Total() >= actual.Total() {
		t.Errorf("200 个 1 字节文件：逻辑 %d 应远小于实际占盘 %d",
			apparent.Total(), actual.Total())
	}
	if !apparent.Apparent {
		t.Error("Result.Apparent 应标记为 true")
	}
}

// 稀疏文件是反方向：逻辑远大于实际。
func TestScan_SparseFileActualIsSmallerThanApparent(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	root := t.TempDir()
	f, err := os.Create(filepath.Join(root, "sparse"))
	if err != nil {
		t.Fatal(err)
	}
	const logical = 64 << 20
	if err := f.Truncate(logical); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	actual := scan(t, root, nil)
	apparent := scan(t, root, func(o *Options) { o.Apparent = true })
	if actual.Total() >= apparent.Total() {
		t.Errorf("稀疏文件：实际 %d 应远小于逻辑 %d", actual.Total(), apparent.Total())
	}
}

func TestScan_ErrorsOnBadInput(t *testing.T) {
	if _, err := Scan(Options{Root: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Error("路径不存在应返回错误")
	}
	root := mktree(t, map[string]int{"f": 10})
	if _, err := Scan(Options{Root: filepath.Join(root, "f")}); err == nil {
		t.Error("参数是文件而非目录应返回错误")
	}
}

func TestScan_EmptyDir(t *testing.T) {
	root := t.TempDir()
	res := scan(t, root, nil)
	if len(res.Nodes) != 1 {
		t.Errorf("空目录应只有根节点，得到 %d 个", len(res.Nodes))
	}
	if res.TotalFiles() != 0 {
		t.Errorf("空目录文件数应为 0，得到 %d", res.TotalFiles())
	}
}

// 拿真实目录树与系统 du 对拍。默认跳过（造不出有代表性的树），设
// JDAN_SIZE_DU_TREE=/some/path 手动跑：
//
//	JDAN_SIZE_DU_TREE=~/Library go test -run MatchesDuOnRealTree ./internal/sizex/ -v
//
// 合成用例覆盖不到真实树里的东西：深层嵌套、大量硬链接（Homebrew/pnpm）、
// 混杂的符号链接、权限受限目录。
func TestScan_MatchesDuOnRealTree(t *testing.T) {
	tree := os.Getenv("JDAN_SIZE_DU_TREE")
	if tree == "" {
		t.Skip("设 JDAN_SIZE_DU_TREE=<path> 启用真实目录对拍")
	}
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	res, err := Scan(Options{Root: tree, OneFileSystem: true, IncludeHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	want := duKiB(t, tree, "-x") * 1024

	t.Logf("jdan=%d 字节 / du=%d 字节 / 目录 %d / 文件 %d / 去重 %d / 权限错误 %d",
		res.Total(), want, len(res.Nodes), res.TotalFiles(), res.Deduped, len(res.Errors))

	// du 报的是 KiB，四舍五入会引入最多 1 KiB × 节点数的量化误差；用相对误差判定。
	diff := int64(res.Total()) - int64(want)
	if diff < 0 {
		diff = -diff
	}
	if want > 0 && float64(diff)/float64(want) > 0.01 {
		t.Errorf("与 du 相差 %d 字节（%.2f%%），超过 1%%", diff, 100*float64(diff)/float64(want))
	}
}

// 根目录自身的 st_blocks 必须计入 —— ext4 上每个目录 4096 B，10 万目录
// 就是 400 MB，不计入的话与 du 的对拍在 Linux 上会失败。
// （APFS 上目录 Blocks=0，所以这条在 macOS 上看不出差别，靠 CI 的 ubuntu 覆盖。）
func TestScan_DirectoriesContributeOwnBlocks(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	flat := mktree(t, map[string]int{"f": 4096})
	nested := mktree(t, map[string]int{"a/b/c/d/e/f": 4096})

	flatRes := scan(t, flat, nil)
	nestedRes := scan(t, nested, nil)

	// 与 du 对拍才是真断言：无论目录占不占块，两边都得一致。
	if want := duKiB(t, nested) * 1024; nestedRes.Total() != want {
		t.Errorf("深层树总量 %d 与 du 的 %d 不符（目录自身块未计入？）", nestedRes.Total(), want)
	}
	t.Logf("扁平树 %d 字节 / 深层树 %d 字节（差值即 5 层目录自身占盘）",
		flatRes.Total(), nestedRes.Total())
}
