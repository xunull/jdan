package gitx

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Contributor 是一个作者的提交统计。
type Contributor struct {
	Name    string  `json:"name"`
	Commits int     `json:"commits"`
	Percent float64 `json:"percent"`
}

// Hotspot 是一个文件的改动次数。
type Hotspot struct {
	Path    string `json:"path"`
	Changes int    `json:"changes"`
}

// Summary 是仓库一眼看的全部信息。
type Summary struct {
	Repo         string        `json:"repo"`
	Commits      int           `json:"commits"`
	Branches     int           `json:"branches"`
	Tags         int           `json:"tags"`
	FirstCommit  string        `json:"first_commit"` // YYYY-MM-DD
	LastCommit   string        `json:"last_commit"`  // YYYY-MM-DD
	Age          string        `json:"age"`          // 人类可读跨度
	Contributors []Contributor `json:"contributors"`
	Hotspots     []Hotspot     `json:"hotspots"`
}

// errEmptyRepo 表示仓库还没有任何提交。
var errEmptyRepo = fmt.Errorf("仓库还没有任何提交")

// Summarize 收集 dir 仓库的概览。top 限制贡献者/hotspots 的条数。
func Summarize(run Runner, dir string, top int) (Summary, error) {
	if !IsRepo(run, dir) {
		return Summary{}, fmt.Errorf("不是 git 仓库（试试 git init）")
	}
	if top < 1 {
		top = 5
	}

	var s Summary

	// 仓库名 = 工作区顶层目录的 basename
	if out, err := run(dir, "rev-parse", "--show-toplevel"); err == nil {
		s.Repo = filepath.Base(strings.TrimSpace(out))
	}

	// commit 数（空仓库时 rev-list 会失败 → 视为 0 提交）
	out, err := run(dir, "rev-list", "--count", "HEAD")
	if err != nil {
		return Summary{}, errEmptyRepo
	}
	s.Commits, _ = strconv.Atoi(strings.TrimSpace(out))
	if s.Commits == 0 {
		return Summary{}, errEmptyRepo
	}

	s.Branches = countLines(run, dir, "for-each-ref", "--format=%(refname)", "refs/heads")
	s.Tags = countLines(run, dir, "tag")

	// 年龄：首 commit 日期 → 末 commit 日期
	if first, err := run(dir, "log", "--reverse", "--format=%cI", "--max-parents=0"); err == nil {
		s.FirstCommit, s.LastCommit, s.Age = computeAge(first, lastCommitISO(run, dir))
	}

	s.Contributors = contributors(run, dir, s.Commits, top)
	s.Hotspots = hotspots(run, dir, top)
	return s, nil
}

func countLines(run Runner, dir string, args ...string) int {
	out, err := run(dir, args...)
	if err != nil {
		return 0
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func lastCommitISO(run Runner, dir string) string {
	out, _ := run(dir, "log", "-1", "--format=%cI")
	return out
}

// computeAge 解析首/末 commit 的 ISO 时间，返回 YYYY-MM-DD 首日、末日、人类可读跨度。
func computeAge(firstISO, lastISO string) (first, last, age string) {
	// firstISO 可能含多行（--max-parents=0 + --reverse 时取第一行）
	firstISO = firstLine(firstISO)
	lastISO = firstLine(lastISO)
	ft, err1 := time.Parse(time.RFC3339, firstISO)
	lt, err2 := time.Parse(time.RFC3339, lastISO)
	if err1 != nil || err2 != nil {
		return "", "", ""
	}
	return ft.Format("2006-01-02"), lt.Format("2006-01-02"), humanizeSpan(ft, lt)
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if head, _, found := strings.Cut(s, "\n"); found {
		return strings.TrimSpace(head)
	}
	return s
}

// humanizeSpan 把两个时间的跨度说成「约 N 年/个月/天」。
func humanizeSpan(from, to time.Time) string {
	if to.Before(from) {
		from, to = to, from
	}
	days := int(to.Sub(from).Hours() / 24)
	switch {
	case days <= 0:
		return "今天"
	case days < 31:
		return fmt.Sprintf("约 %d 天", days)
	case days < 365:
		return fmt.Sprintf("约 %d 个月", days/30)
	default:
		years := days / 365
		months := (days % 365) / 30
		if months == 0 {
			return fmt.Sprintf("约 %d 年", years)
		}
		return fmt.Sprintf("约 %d 年 %d 个月", years, months)
	}
}

func contributors(run Runner, dir string, total, top int) []Contributor {
	out, err := run(dir, "log", "--format=%an")
	if err != nil {
		return nil
	}
	counts := map[string]int{}
	for name := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		name = strings.TrimSpace(name)
		if name != "" {
			counts[name]++
		}
	}
	list := make([]Contributor, 0, len(counts))
	for name, c := range counts {
		pct := 0.0
		if total > 0 {
			pct = float64(c) / float64(total) * 100
		}
		list = append(list, Contributor{Name: name, Commits: c, Percent: pct})
	}
	// 按提交数降序，同数按名字升序（稳定）
	sort.Slice(list, func(i, j int) bool {
		if list[i].Commits != list[j].Commits {
			return list[i].Commits > list[j].Commits
		}
		return list[i].Name < list[j].Name
	})
	if len(list) > top {
		list = list[:top]
	}
	return list
}

func hotspots(run Runner, dir string, top int) []Hotspot {
	out, err := run(dir, "log", "--pretty=format:", "--name-only")
	if err != nil {
		return nil
	}
	counts := map[string]int{}
	for path := range strings.SplitSeq(out, "\n") {
		path = strings.TrimSpace(path)
		if path != "" {
			counts[path]++
		}
	}
	list := make([]Hotspot, 0, len(counts))
	for path, c := range counts {
		list = append(list, Hotspot{Path: path, Changes: c})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Changes != list[j].Changes {
			return list[i].Changes > list[j].Changes
		}
		return list[i].Path < list[j].Path
	})
	if len(list) > top {
		list = list[:top]
	}
	return list
}
