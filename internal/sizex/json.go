package sizex

import (
	"path/filepath"
	"sort"
)

// JSONNode 是 --json 输出的节点。
//
// type 字段是必需的：默认模式下文件体积折叠进所属目录、只有 --files 才有
// 文件节点，没有这个字段消费者无从判断 children 里的项是目录还是文件。
type JSONNode struct {
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "dir" | "file"
	Bytes    uint64      `json:"bytes"`
	Files    uint64      `json:"files"`
	Children []*JSONNode `json:"children"`
}

// JSONRoot 是 --json 的顶层对象。apparent / supported 只出现在根，
// 子节点不重复。
type JSONRoot struct {
	*JSONNode
	Apparent  bool        `json:"apparent"`
	Supported bool        `json:"supported"`
	Deduped   int         `json:"deduped"`
	Errors    []JSONError `json:"errors"`
}

type JSONError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// JSONData 把扫描结果转成可序列化的树。
//
// 刻意**不接受** Depth / Top：--json 永远输出全树。那两个是展示层的裁剪，
// 若让它们影响 JSON，「连跑 3 次逐字节相同」这条验收覆盖的东西就会随 flag
// 漂移，也让下游消费者拿不到完整数据。
func (r *Result) JSONData() *JSONRoot {
	kidsOf := make(map[string][]*Node, len(r.Nodes))
	for p, n := range r.Nodes {
		if p == r.Root {
			continue
		}
		kidsOf[filepath.Dir(p)] = append(kidsOf[filepath.Dir(p)], n)
	}

	root := r.Nodes[r.Root]
	if root == nil {
		root = &Node{Path: r.Root}
	}

	errs := make([]JSONError, 0, len(r.Errors))
	for _, e := range r.SortedErrors() {
		errs = append(errs, JSONError{Path: e.Path, Error: e.Err.Error()})
	}

	return &JSONRoot{
		JSONNode:  jsonSubtree(root, kidsOf),
		Apparent:  r.Apparent,
		Supported: r.Supported,
		Deduped:   r.Deduped,
		Errors:    errs,
	}
}

func jsonSubtree(n *Node, kidsOf map[string][]*Node) *JSONNode {
	kids := kidsOf[n.Path]
	out := &JSONNode{
		Path:     n.Path,
		Type:     "dir",
		Bytes:    n.Bytes,
		Files:    n.Files,
		Children: make([]*JSONNode, 0, len(kids)),
	}
	for _, k := range kids {
		out.Children = append(out.Children, jsonSubtree(k, kidsOf))
	}
	// 与文本输出同一套排序规则：(Bytes 降序, Path 升序)。不排的话 children
	// 的顺序来自 map 遍历，连跑两次 JSON 就不逐字节相同了。
	sort.Slice(out.Children, func(i, j int) bool {
		if out.Children[i].Bytes != out.Children[j].Bytes {
			return out.Children[i].Bytes > out.Children[j].Bytes
		}
		return out.Children[i].Path < out.Children[j].Path
	})
	return out
}
