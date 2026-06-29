package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xunull/jdan/internal/commitlint"
	"github.com/xunull/jdan/internal/gitx"
)

type commitlintDeps struct {
	out io.Writer
	in  io.Reader
	run gitx.Runner // 注入；nil → 真实 git
	dir string      // 仓库目录；"" → 当前目录
}

// labeledMsg 是一条带来源标签的提交信息（标签用于多条时定位是哪条）。
type labeledMsg struct {
	Label string
	Msg   string
}

func newCommitlintCommand(deps commitlintDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	if deps.run == nil {
		deps.run = gitx.ExecRunner
	}
	if deps.dir == "" {
		deps.dir = "."
	}

	cmd := &cobra.Command{
		Use:   "commitlint [revision-range]",
		Short: "按 Conventional Commits 规范校验提交信息",
		Long: `按 Conventional Commits 规范校验提交信息（type(scope): subject）。0 新依赖。

输入来源（按优先级）：
  -m "feat: x"          直接传字面量
  -f <file>             读文件（commit-msg hook：git 把信息文件路径传进来）
  <revision-range>      校验这些提交（如 HEAD、origin/main..HEAD），底层调 git
  （管道）               从 stdin 读一条
  无参数                 校验 HEAD（最后一条提交）

例：
  jdan git commitlint                          # 校验 HEAD
  jdan git commitlint origin/main..HEAD        # 校验 PR 分支上的全部提交
  jdan git commitlint -m "feat(api): 加分页"    # 校验字面量
  git log -1 --format=%B | jdan git commitlint # 管道
  jdan git commitlint -f "$1"                  # 作为 .git/hooks/commit-msg

退出码：全合规 0、有违规非 0（可直接当 commit-msg hook 拦下）；--warn 只报不拦。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			literal, _ := cmd.Flags().GetString("message")
			typesCSV, _ := cmd.Flags().GetString("types")
			maxHeader, _ := cmd.Flags().GetInt("max-header")
			scopeReq, _ := cmd.Flags().GetBool("scope-required")
			asJSON, _ := cmd.Flags().GetBool("json")
			warnOnly, _ := cmd.Flags().GetBool("warn")

			opts := commitlint.Options{MaxHeaderLen: maxHeader, ScopeRequired: scopeReq}
			if typesCSV != "" {
				opts.Types = splitCSV(typesCSV)
			}

			msgs, err := collectMessages(deps, args, file, literal)
			if err != nil {
				return err
			}
			if len(msgs) == 0 {
				return fmt.Errorf("没有可校验的提交信息")
			}

			type result struct {
				Label      string                 `json:"label"`
				Commit     commitlint.Commit      `json:"commit"`
				Violations []commitlint.Violation `json:"violations"`
			}
			var results []result
			bad := 0
			for _, m := range msgs {
				c := commitlint.Parse(m.Msg)
				vs := commitlint.Lint(c, opts)
				if len(vs) > 0 {
					bad++
				}
				results = append(results, result{Label: m.Label, Commit: c, Violations: vs})
			}

			if asJSON {
				enc := json.NewEncoder(deps.out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(struct {
					OK      bool        `json:"ok"`
					Checked int         `json:"checked"`
					Bad     int         `json:"bad"`
					Results interface{} `json:"results"`
				}{OK: bad == 0, Checked: len(msgs), Bad: bad, Results: results}); err != nil {
					return err
				}
			} else {
				for _, r := range results {
					if len(r.Violations) == 0 {
						fmt.Fprintf(deps.out, "✓ %s  %s\n", r.Label, r.Commit.Header)
						continue
					}
					fmt.Fprintf(deps.out, "✗ %s  %s\n", r.Label, r.Commit.Header)
					for _, v := range r.Violations {
						fmt.Fprintf(deps.out, "    · [%s] %s\n", v.Rule, v.Msg)
					}
				}
				if bad == 0 {
					fmt.Fprintf(deps.out, "\n%d 条提交全部合规 ✓\n", len(msgs))
				} else {
					fmt.Fprintf(deps.out, "\n%d/%d 条提交不合规 ✗\n", bad, len(msgs))
				}
			}

			if bad > 0 && !warnOnly {
				return fmt.Errorf("%d 处提交信息不合规", bad)
			}
			return nil
		},
	}
	cmd.Flags().StringP("file", "f", "", "从文件读取提交信息（commit-msg hook 用）")
	cmd.Flags().StringP("message", "m", "", "直接校验一条字面量信息")
	cmd.Flags().String("types", "", "覆盖允许的 type 白名单（逗号分隔）")
	cmd.Flags().Int("max-header", commitlint.DefaultMaxHeader, "header 长度上限（按字符/rune）")
	cmd.Flags().Bool("scope-required", false, "强制要求有 scope")
	cmd.Flags().Bool("json", false, "JSON 输出")
	cmd.Flags().Bool("warn", false, "软模式：报告违规但仍退出 0（不拦）")
	return cmd
}

// collectMessages 按优先级取要校验的提交信息：-m > -f > revision-range > stdin > HEAD。
func collectMessages(deps commitlintDeps, args []string, file, literal string) ([]labeledMsg, error) {
	switch {
	case literal != "":
		return []labeledMsg{{Label: "-m", Msg: literal}}, nil
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return []labeledMsg{{Label: file, Msg: string(b)}}, nil
	case len(args) == 1:
		return gitRangeMessages(deps, args[0])
	}
	// 没有 -m/-f/位置参数：stdin 是管道就读 stdin，否则默认校验 HEAD
	if isPiped(deps.in) {
		b, err := io.ReadAll(deps.in)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(string(b)) != "" {
			return []labeledMsg{{Label: "stdin", Msg: string(b)}}, nil
		}
	}
	return gitRangeMessages(deps, "HEAD")
}

// gitRangeMessages 用 git log 取一个 revision/range 里每条提交的信息（短 hash 作标签）。
func gitRangeMessages(deps commitlintDeps, rev string) ([]labeledMsg, error) {
	// %h<US>%B<RS>：US(0x1f) 分隔 hash 与正文，RS(0x1e) 分隔提交
	out, err := deps.run(deps.dir, "log", "--no-color", "--format=%h%x1f%B%x1e", rev)
	if err != nil {
		return nil, err
	}
	var msgs []labeledMsg
	for rec := range strings.SplitSeq(out, "\x1e") {
		rec = strings.Trim(rec, "\n")
		if rec == "" {
			continue
		}
		hash, body, _ := strings.Cut(rec, "\x1f")
		msgs = append(msgs, labeledMsg{Label: hash, Msg: body})
	}
	return msgs, nil
}

func isPiped(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return true // 测试注入的非 *os.File reader：当作管道读
	}
	return !term.IsTerminal(int(f.Fd()))
}

func splitCSV(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
