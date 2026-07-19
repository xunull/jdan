package sizex

import (
	"path/filepath"
	"sort"
	"strconv"
)

// Tree 是渲染用的嵌套结构。由 BuildTree 从扁平的 Result.Nodes 一次性拼出，
// 单线程纯函数。
type Tree struct {
	Path  string
	Name  string // 相对父级的显示名；根节点是完整路径
	Bytes uint64
	Files uint64
	Kids  []*Tree

	// Aggregated>0 表示这是「其他 N 项」合成行，代表被 --top 截掉的 N 个兄弟。
	// 它永远排在同层末尾且不参与排序（见 sortKids）。
	Aggregated int
}

// TreeOptions 控制拼树时的裁剪。
type TreeOptions struct {
	Depth int // 展开层数；1 = 根 + 直接子项。<=0 视为 1
	Top   int // 每层最多显示几项，其余合并为「其他 N 项」。<=0 表示不限
}

// BuildTree 把扁平节点表拼成嵌套树，并按确定性规则排序、裁剪。
//
// 排序规则（设计文档 §12）：同层按 (Bytes 降序, Name 升序)。Name 兜底是
// 必须的 —— 只按 Bytes 排的话，体积相同的兄弟顺序取决于 map 遍历顺序，
// 并发扫描后同一棵树跑两次 JSON 就不逐字节相同了。同目录内文件名唯一，
// 所以 (Bytes, Name) 是全序。
func BuildTree(r *Result, opts TreeOptions) *Tree {
	if opts.Depth <= 0 {
		opts.Depth = 1
	}

	// 先按父路径归拢直接子节点。
	kidsOf := make(map[string][]*Node, len(r.Nodes))
	for p, n := range r.Nodes {
		if p == r.Root {
			continue
		}
		parent := filepath.Dir(p)
		kidsOf[parent] = append(kidsOf[parent], n)
	}

	root := r.Nodes[r.Root]
	if root == nil {
		return &Tree{Path: r.Root, Name: r.Root}
	}
	return buildSubtree(root, r.Root, kidsOf, opts, 0)
}

func buildSubtree(n *Node, root string, kidsOf map[string][]*Node, opts TreeOptions, depth int) *Tree {
	name := n.Path
	if n.Path != root {
		name = filepath.Base(n.Path)
	}
	t := &Tree{Path: n.Path, Name: name, Bytes: n.Bytes, Files: n.Files}

	if depth >= opts.Depth {
		return t // 到达展示深度，不再展开子项（子项体积已经算进 Bytes 了）
	}

	kids := kidsOf[n.Path]
	if len(kids) == 0 {
		return t
	}

	sub := make([]*Tree, 0, len(kids))
	for _, k := range kids {
		sub = append(sub, buildSubtree(k, root, kidsOf, opts, depth+1))
	}
	sortKids(sub)

	// --top 截断：用同一个全序比较器排完再截，边界因此确定 —— 第 N 名和
	// 第 N+1 名 Bytes 相同时，Name 决定谁进榜，不会随运行变化。
	if opts.Top > 0 && len(sub) > opts.Top {
		var restBytes, restFiles uint64
		for _, k := range sub[opts.Top:] {
			restBytes += k.Bytes
			restFiles += k.Files
		}
		cut := len(sub) - opts.Top
		sub = append(sub[:opts.Top:opts.Top], &Tree{
			Path:       n.Path,
			Bytes:      restBytes,
			Files:      restFiles,
			Aggregated: cut,
		})
	}
	t.Kids = sub
	return t
}

// sortKids 按 (Bytes 降序, Name 升序) 排序。全序，与输入顺序无关。
//
// 用 sort.Slice 而非 SliceStable 是可以的，正因为比较器是全序：输入顺序
// 本身就不确定（来自 map 遍历），稳定排序保不住任何东西，只能靠比较器消歧。
func sortKids(ts []*Tree) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Bytes != ts[j].Bytes {
			return ts[i].Bytes > ts[j].Bytes
		}
		return ts[i].Name < ts[j].Name
	})
}

// SortedErrors 返回按路径升序排好的扫描错误。
//
// 并发扫描时错误是各 worker 往共享 slice 里 append 的，顺序不确定；不排序
// 的话 --json 的「连跑 3 次逐字节相同」会挂在这里。
func (r *Result) SortedErrors() []ScanError {
	out := make([]ScanError, len(r.Errors))
	copy(out, r.Errors)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// RelPath 返回节点相对扫描根的路径，根本身返回 "."。
func (t *Tree) RelPath(root string) string {
	if t.Path == root {
		return "."
	}
	rel, err := filepath.Rel(root, t.Path)
	if err != nil {
		return t.Path
	}
	return rel
}

// displayName 给渲染层用：聚合行显示「其他 N 项」，其余显示名字。
func (t *Tree) displayName() string {
	if t.Aggregated > 0 {
		return "其他 " + strconv.Itoa(t.Aggregated) + " 项"
	}
	return t.Name
}
