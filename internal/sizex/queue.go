package sizex

import "sync"

// dirQueue 是并发扫描用的无界目录队列，带终止检测。
//
// 为什么不用信号量：如果让 goroutine「持有配额的同时阻塞等待子目录获取
// 配额」，N 个父目录会占满全部 N 个配额、各自等待永远启动不了的子目录 →
// 死锁。这是并发目录遍历的经典失败模式。扁平队列 + 固定 N 个 worker 在
// 结构上就不存在递归获取配额，因而不可能死锁。worker 数本身就是限流，
// 不需要额外的信号量（也就不需要 golang.org/x/sync）。
//
// 为什么必须无界：换成有界 channel 的话，所有 worker 同时阻塞在 push
// 子目录上就是死锁 —— 换了个形状的同一个 bug。
//
// 终止检测（outstanding 计数器）是这里最容易写错的地方，见 push/done。
type dirQueue struct {
	mu    sync.Mutex
	cond  *sync.Cond
	items []*Node

	// outstanding 是「已入队但尚未处理完」的目录数。它不等于 len(items)：
	// 一个目录被 pop 出去之后、ReadDir 还没跑完之前，它不在 items 里，但
	// 仍然可能 push 出新的子目录，所以还得算作未完成。
	//
	// 计数器的自增点是这个方案唯一容易写反的地方：
	//   push 时 +1（根目录入队时也走 push，所以初值是 0）
	//   pop  时不动
	//   done 时 -1（ReadDir 与子目录全部 push 完之后才调）
	// 漏了 push 时 +1 → 提前退出，静默少扫；改成 pop 时 +1 → 永不归零，挂死。
	outstanding int
	closed      bool
}

func newDirQueue() *dirQueue {
	q := &dirQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// push 入队待扫描目录。永不阻塞（无界）。
func (q *dirQueue) push(dirs ...*Node) {
	if len(dirs) == 0 {
		return
	}
	q.mu.Lock()
	q.items = append(q.items, dirs...)
	q.outstanding += len(dirs)
	q.mu.Unlock()
	q.cond.Broadcast()
}

// pop 取一个目录。队列暂时为空但仍有未完成目录时阻塞等待；全部完成时
// 返回 ok=false 让 worker 退出。
func (q *dirQueue) pop() (*Node, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		if n := len(q.items); n > 0 {
			d := q.items[n-1]
			q.items = q.items[:n-1]
			return d, true
		}
		if q.closed || q.outstanding == 0 {
			// 队列空且无人还在产出 → 真的干完了。
			return nil, false
		}
		// 队列此刻为空，但还有 worker 正在 ReadDir，随时可能 push 出新目录。
		// 这两种情况必须区分开，否则会静默少扫一大片 —— 症状是「数字偏小」
		// 而不是挂起，很容易被误当成权限问题。
		q.cond.Wait()
	}
}

// done 标记一个目录处理完毕（ReadDir 完成、子目录已全部 push）。
// 计数归零时唤醒所有等待中的 worker 退出。
func (q *dirQueue) done() {
	q.mu.Lock()
	q.outstanding--
	last := q.outstanding == 0
	q.mu.Unlock()
	if last {
		q.cond.Broadcast()
	}
}
