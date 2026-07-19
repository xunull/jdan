package sizex

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Options 控制一次扫描。
type Options struct {
	Root          string // 起始目录
	Apparent      bool   // 用逻辑大小（Size()）而非实际占盘
	OneFileSystem bool   // 不跨越文件系统边界（对齐 du -x）
	IncludeHidden bool   // 含以 . 开头的条目
	Jobs          int    // 并发度；<=1 为单线程
}

// Node 是树上的一个目录。文件不单独建节点，体积折叠进所属目录
// （默认输出只列目录，为文件建节点会让 100 万文件的树多占 10-50 倍内存）。
type Node struct {
	Path  string
	Bytes uint64 // rollup 后：整棵子树的占盘
	Files uint64 // rollup 后：整棵子树的文件数

	// selfBytes 是「本目录自身的 st_blocks + 直属文件的占盘」，不含子目录。
	// rollup 时才把子目录累加上来。这样硬链接 Pass 2 只需要改直属父目录，
	// 不用逐层往上爬祖先。
	selfBytes uint64
	selfFiles uint64
}

// ScanError 是扫描中遇到的单点失败（通常是 EACCES）。不中断扫描。
type ScanError struct {
	Path string
	Err  error
}

// Result 是一次扫描的完整结果。Nodes 是扁平的 path → 目录节点，
// 渲染前用 BuildTree 拼成树。
type Result struct {
	Root      string
	Nodes     map[string]*Node
	Errors    []ScanError
	Apparent  bool // 数字是逻辑大小而非实际占盘
	Supported bool // 本平台能拿到 st_blocks / inode；false 时无去重、无跨盘检测
	Deduped   int  // 因硬链接去重而未重复计入的文件数（诊断用）
}

// deferredLink 是一个 nlink>1 的文件，Pass 1 只收集不计入。
type deferredLink struct {
	key   inodeKey
	bytes uint64
	path  string
}

// inodeKey 是硬链接去重的键。dev 必须是键的一部分：不同设备上 inode 号会重复。
type inodeKey struct {
	dev, ino uint64
}

// sizeOf 按 Apparent 选项返回该条目计入统计的字节数，以及 dev/ino/nlink。
func (o Options) sizeOf(info os.FileInfo) (bytes, dev, ino, nlink uint64, ok bool) {
	bytes, dev, ino, nlink, ok = Stat(info)
	if o.Apparent {
		bytes = uint64(info.Size())
	}
	return bytes, dev, ino, nlink, ok
}

// Scan 遍历 opts.Root 下的整棵树，返回扁平的目录节点表。
//
// 语义对齐 du：目录自身的 st_blocks 计入；硬链接只计一次；默认不跨文件系统；
// 不跟随符号链接（链接自身按其占盘计）。权限错误收集进 Errors 不中断。
//
// 硬链接归属与 du 不同：du 归给「先遇到的」那条路径，并发下不确定；这里归给
// **字典序最小的路径**，保证同一棵树的输出可复现。根总量两者一致，单个子目录
// 的数字可能不同。
func Scan(opts Options) (*Result, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}
	// 起始参数是符号链接时跟随它 —— 用户敲 `jdan size ~/somelink` 的意图是看
	// 目标目录。树内部的符号链接仍然不跟随。
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("无法读取 %q：%w", opts.Root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q 不是目录", opts.Root)
	}

	rootBytes, rootDev, _, _, supported := opts.sizeOf(info)

	res := &Result{
		Root:      root,
		Nodes:     map[string]*Node{root: {Path: root, selfBytes: rootBytes}},
		Apparent:  opts.Apparent,
		Supported: supported,
	}

	var deferred []deferredLink
	stack := []string{root}

	for len(stack) > 0 {
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		entries, err := os.ReadDir(dir)
		if err != nil {
			res.Errors = append(res.Errors, ScanError{Path: dir, Err: err})
			continue
		}
		node := res.Nodes[dir]

		for _, e := range entries {
			name := e.Name()
			if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			p := filepath.Join(dir, name)

			// Lstat 而非 Stat：符号链接量它自己，不跟随（否则会重复计数并可能绕环）。
			info, err := os.Lstat(p)
			if err != nil {
				res.Errors = append(res.Errors, ScanError{Path: p, Err: err})
				continue
			}
			bytes, dev, ino, nlink, _ := opts.sizeOf(info)

			if info.IsDir() {
				// 跨文件系统检测只在目录边界做：挂载点一定是目录，文件不可能与
				// 其所在目录不同设备。
				if opts.OneFileSystem && supported && dev != rootDev {
					continue
				}
				res.Nodes[p] = &Node{Path: p, selfBytes: bytes}
				stack = append(stack, p)
				continue
			}

			node.selfFiles++
			if supported && nlink > 1 {
				// Pass 1 只收集，不计入任何目录。归属留到 Pass 2 确定性地定。
				// 只有 nlink>1 才进队列：绝大多数文件 nlink==1，这道门槛能跳过
				// 90%+ 的队列写入。
				deferred = append(deferred, deferredLink{
					key:   inodeKey{dev: dev, ino: ino},
					bytes: bytes,
					path:  p,
				})
				continue
			}
			node.selfBytes += bytes
		}
	}

	res.attributeHardlinks(deferred)
	res.rollup()
	return res, nil
}

// attributeHardlinks 是 Pass 2：把每组同 inode 的文件归属给字典序最小的路径。
//
// 之所以不能沿用 du 的「先遇到的算」：并发扫描下「先遇到」取决于调度，同一
// 棵树跑两次各子树的数字会互换。字典序最小与遍历顺序无关（Go 的 string <
// 按字节比较，是全序），因此并发与单线程结果逐字节一致。
//
// 只需要加到直属父目录的 selfBytes 上 —— rollup 会把它传播给所有祖先。
func (r *Result) attributeHardlinks(deferred []deferredLink) {
	if len(deferred) == 0 {
		return
	}
	best := make(map[inodeKey]deferredLink, len(deferred))
	for _, d := range deferred {
		if cur, ok := best[d.key]; !ok || d.path < cur.path {
			best[d.key] = d
		}
	}
	r.Deduped = len(deferred) - len(best)

	for _, d := range best {
		if parent := r.Nodes[filepath.Dir(d.path)]; parent != nil {
			parent.selfBytes += d.bytes
		}
	}
}

// rollup 把 selfBytes/selfFiles 自底向上累加成子树总量。
//
// 按路径长度降序处理即可保证「子先于父」：子路径必然严格长于父路径（父是它的
// 前缀）。长度相同的按字典序，纯粹为了确定性。
func (r *Result) rollup() {
	paths := make([]string, 0, len(r.Nodes))
	for p := range r.Nodes {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) != len(paths[j]) {
			return len(paths[i]) > len(paths[j])
		}
		return paths[i] < paths[j]
	})

	for _, p := range paths {
		n := r.Nodes[p]
		n.Bytes += n.selfBytes
		n.Files += n.selfFiles
		if p == r.Root {
			continue
		}
		if parent := r.Nodes[filepath.Dir(p)]; parent != nil {
			parent.Bytes += n.Bytes
			parent.Files += n.Files
		}
	}
}

// Total 返回根节点的子树总量。
func (r *Result) Total() uint64 {
	if n := r.Nodes[r.Root]; n != nil {
		return n.Bytes
	}
	return 0
}

// TotalFiles 返回扫到的文件总数。
func (r *Result) TotalFiles() uint64 {
	if n := r.Nodes[r.Root]; n != nil {
		return n.Files
	}
	return 0
}
