package sizex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// wideDeepTree 造一棵又深又宽的树，专门用来暴露终止检测缺陷：
// 队列会反复出现「此刻为空但还有 worker 正在 ReadDir」的窗口。
func wideDeepTree(t *testing.T, breadth, depth int) string {
	t.Helper()
	root := t.TempDir()
	var build func(dir string, level int)
	build = func(dir string, level int) {
		if level == 0 {
			return
		}
		for i := range breadth {
			sub := filepath.Join(dir, fmt.Sprintf("d%02d", i))
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(sub, "f"), make([]byte, 1024*(i+1)), 0o644); err != nil {
				t.Fatal(err)
			}
			build(sub, level-1)
		}
	}
	build(root, depth)
	return root
}

// 核心验收：--jobs 16 与 --jobs 1 结果逐字节相同，重复 100 次。
//
// 100 次不是凑数：终止检测缺陷的表现是概率性的「静默少扫」——症状是数字
// 偏小而不是挂起，跑一两次很可能碰不到。
func TestScan_ConcurrentMatchesSingleThreaded(t *testing.T) {
	root := wideDeepTree(t, 6, 4)

	single, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(single.JSONData())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("基准：%d 字节 / %d 目录 / %d 文件", single.Total(), len(single.Nodes), single.TotalFiles())

	for i := range 100 {
		got, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 16})
		if err != nil {
			t.Fatalf("第 %d 次并发扫描出错：%v", i, err)
		}
		if got.Total() != single.Total() {
			t.Fatalf("第 %d 次总量 %d != 单线程 %d（少扫了 %d 字节）",
				i, got.Total(), single.Total(), int64(single.Total())-int64(got.Total()))
		}
		if len(got.Nodes) != len(single.Nodes) {
			t.Fatalf("第 %d 次目录数 %d != 单线程 %d（终止检测提前退出？）",
				i, len(got.Nodes), len(single.Nodes))
		}
		b, err := json.Marshal(got.JSONData())
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != string(want) {
			t.Fatalf("第 %d 次 JSON 与单线程不逐字节相同", i)
		}
	}
}

// 并发度逐档扫描，结果都必须一致。
func TestScan_AllJobCountsAgree(t *testing.T) {
	root := wideDeepTree(t, 5, 3)

	var want uint64
	for _, jobs := range []int{1, 2, 3, 4, 8, 16, 32, 64} {
		res, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: jobs})
		if err != nil {
			t.Fatalf("jobs=%d: %v", jobs, err)
		}
		if want == 0 {
			want = res.Total()
			continue
		}
		if res.Total() != want {
			t.Errorf("jobs=%d 总量 %d != %d", jobs, res.Total(), want)
		}
	}
}

// 并发度远大于目录数时不能挂死（worker 比活多，大部分要在 cond 上等）。
func TestScan_MoreWorkersThanDirsTerminates(t *testing.T) {
	root := mktree(t, map[string]int{"only/f": 1024})
	res, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 64})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 2 { // root + only
		t.Errorf("应有 2 个目录节点，得到 %d", len(res.Nodes))
	}
}

// 空目录 + 高并发：outstanding 从 1 直接归零，不能死等。
func TestScan_EmptyDirWithManyWorkersTerminates(t *testing.T) {
	res, err := Scan(Options{Root: t.TempDir(), OneFileSystem: true, Jobs: 32})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Nodes) != 1 {
		t.Errorf("空目录应只有根节点，得到 %d", len(res.Nodes))
	}
}

// 并发下硬链接归属仍然确定：跑 50 次，各子树数字不能变。
// 这是两趟归属方案存在的唯一理由 —— du 的「先遇到的算」在这里会翻车。
func TestScan_ConcurrentHardlinkAttributionStable(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 inode 信息")
	}
	root := t.TempDir()
	// 多个目录，每个都放一条指向同一 inode 的硬链接
	orig := filepath.Join(root, "aaa", "orig")
	if err := os.MkdirAll(filepath.Dir(orig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orig, make([]byte, 1<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"bbb", "ccc", "ddd", "eee", "fff", "ggg"} {
		sub := filepath.Join(root, d)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(orig, filepath.Join(sub, "link")); err != nil {
			t.Skipf("不支持硬链接：%v", err)
		}
	}

	first, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 16})
	if err != nil {
		t.Fatal(err)
	}
	for i := range 50 {
		got, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 16})
		if err != nil {
			t.Fatal(err)
		}
		for p, n := range first.Nodes {
			if got.Nodes[p].Bytes != n.Bytes {
				t.Fatalf("第 %d 次并发扫描 %s 的 Bytes 变了：%d → %d（归属不确定）",
					i, p, n.Bytes, got.Nodes[p].Bytes)
			}
		}
	}
	// 1 MiB 必须落在字典序最小的 aaa 下，而不是随机某个目录
	if at(t, first, "aaa").Bytes < 1<<20 {
		t.Errorf("硬链接应归给字典序最小的 aaa，其 Bytes=%d", at(t, first, "aaa").Bytes)
	}
}

// 并发下权限错误也要全部收齐，且顺序经 SortedErrors 归一。
func TestScan_ConcurrentErrorsAreAllCollected(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root 不受权限限制")
	}
	root := t.TempDir()
	var denied []string
	for i := range 12 {
		d := filepath.Join(root, fmt.Sprintf("denied%02d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(d, 0o000); err != nil {
			t.Skip(err)
		}
		denied = append(denied, d)
	}
	t.Cleanup(func() {
		for _, d := range denied {
			os.Chmod(d, 0o755)
		}
	})

	res, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 16})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 12 {
		t.Errorf("应收集到 12 条权限错误，得到 %d", len(res.Errors))
	}
	sorted := res.SortedErrors()
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].Path > sorted[i].Path {
			t.Errorf("SortedErrors 未按路径升序：%s 在 %s 之前", sorted[i-1].Path, sorted[i].Path)
		}
	}
}

// 进度计数器由调用方轮询，因此必须单调不减。
//
// 早先设计成 push 式回调 Progress(n)，-race 下当场挂了：多个 worker 各自
// 原子 Add 之后再调回调，拿到 161 的那个可能先于拿到 160 的那个执行，显示
// 上进度会往回跳。改成调用方轮询原子计数器后天然单调。
func TestScan_ScannedCounterIsMonotonic(t *testing.T) {
	root := wideDeepTree(t, 4, 3)

	var scanned atomic.Uint64
	stop := make(chan struct{})
	bad := make(chan string, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var last uint64
		for {
			select {
			case <-stop:
				return
			default:
			}
			n := scanned.Load()
			if n < last {
				select {
				case bad <- fmt.Sprintf("进度计数回退：%d → %d", last, n):
				default:
				}
				return
			}
			last = n
		}
	}()

	res, err := Scan(Options{Root: root, OneFileSystem: true, Jobs: 8, Scanned: &scanned})
	close(stop)
	wg.Wait()

	if err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-bad:
		t.Error(msg)
	default:
	}
	if scanned.Load() == 0 {
		t.Error("Scanned 计数器应被累加")
	}
	// 计数是「已扫条目数」，含目录和文件，应不小于文件数
	if scanned.Load() < res.TotalFiles() {
		t.Errorf("已扫条目 %d 不应小于文件数 %d", scanned.Load(), res.TotalFiles())
	}
}

// 队列本身的终止语义：outstanding 归零后 pop 必须返回 false 而不是挂死。
func TestDirQueue_TerminatesWhenOutstandingHitsZero(t *testing.T) {
	q := newDirQueue()
	q.push(&Node{Path: "/a"})

	done := make(chan struct{})
	go func() {
		defer close(done)
		n, ok := q.pop()
		if !ok || n.Path != "/a" {
			t.Errorf("首次 pop 应拿到 /a，得到 %v %v", n, ok)
			return
		}
		q.done() // 处理完，且没有推出子目录
		if _, ok := q.pop(); ok {
			t.Error("outstanding 归零后 pop 应返回 false")
		}
	}()

	select {
	case <-done:
	case <-timeoutAfter():
		t.Fatal("队列未能终止（终止检测挂死）")
	}
}

// 队列必须区分「此刻为空」和「全部干完」：一个 worker 拿走最后一项、
// 还没 push 子目录时，另一个 worker 不能就此退出。
func TestDirQueue_EmptyButOutstandingDoesNotTerminate(t *testing.T) {
	q := newDirQueue()
	q.push(&Node{Path: "/a"})

	// worker1 取走唯一一项，此时队列为空但 outstanding=1
	n, ok := q.pop()
	if !ok {
		t.Fatal("应取到 /a")
	}

	// worker2 此刻 pop 必须阻塞，不能返回 false
	got := make(chan bool, 1)
	go func() {
		_, ok := q.pop()
		got <- ok
	}()

	select {
	case ok := <-got:
		t.Fatalf("队列空但 outstanding=1 时 pop 不该返回（返回了 ok=%v，会静默少扫）", ok)
	case <-shortPause():
		// 正确：仍在阻塞
	}

	// worker1 推出子目录并完成 → worker2 应拿到子目录
	q.push(&Node{Path: "/a/sub"})
	q.done()
	_ = n

	select {
	case ok := <-got:
		if !ok {
			t.Error("worker2 应拿到 /a/sub")
		}
	case <-timeoutAfter():
		t.Fatal("worker2 未被唤醒")
	}
}

func timeoutAfter() <-chan time.Time { return time.After(5 * time.Second) }
func shortPause() <-chan time.Time   { return time.After(100 * time.Millisecond) }
