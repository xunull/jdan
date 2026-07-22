package tradx

// Change 是一处改动段：pos 是在**输入**里的 rune 偏移，Orig/Conv 是对应的输入段/输出段。
type Change struct {
	Pos  int    `json:"pos"`
	Orig string `json:"orig"`
	Conv string `json:"conv"`
}

// diffCellBudget 是 LCS 表的格子上限。超过则退化成"整行一段"，避免超长单行 OOM。
const diffCellBudget = 4 << 20 // 约 400 万格

// Diff 对原文与译文做 rune 级 LCS diff，返回改动段（供 --diff/--json）。
// 与转换的分趟无关：直接比 in 与 out，pos 用输入侧 rune 偏移。
func Diff(in, out string) []Change {
	a := []rune(in)
	b := []rune(out)
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return nil
	}
	// 超长单行：不建 LCS 表，整体作一段（若确有差异）。
	if n > 0 && m > 0 && n*m > diffCellBudget {
		if in == out {
			return nil
		}
		return []Change{{Pos: 0, Orig: in, Conv: out}}
	}

	// dp[i][j] = a[i:] 与 b[j:] 的 LCS 长度。
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var changes []Change
	var orig, conv []rune
	start := -1 // 当前改动段在输入侧的起始 rune 下标；-1 表示无未决段
	flush := func() {
		if len(orig) > 0 || len(conv) > 0 {
			changes = append(changes, Change{Pos: start, Orig: string(orig), Conv: string(conv)})
		}
		orig, conv, start = orig[:0], conv[:0], -1
	}

	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] { // 公共字：结束当前改动段
			flush()
			i++
			j++
			continue
		}
		if start < 0 {
			start = i
		}
		// 删 a[i] 还是插 b[j]，累积进同一段，直到下一个公共字才 flush。
		if dp[i+1][j] >= dp[i][j+1] {
			orig = append(orig, a[i])
			i++
		} else {
			conv = append(conv, b[j])
			j++
		}
	}
	if i < n || j < m { // 尾部纯删/纯插
		if start < 0 {
			start = i
		}
		orig = append(orig, a[i:n]...)
		conv = append(conv, b[j:m]...)
	}
	flush()
	return changes
}
