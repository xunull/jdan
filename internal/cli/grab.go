package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/grabx"
)

type grabCmdDeps struct {
	out io.Writer
	in  io.Reader
}

// grabOrder 固定类型顺序；grabExtractors 把类型名映射到抽取函数。
var grabOrder = []string{"url", "email", "ip"}
var grabExtractors = map[string]func(string) []string{
	"url":   grabx.URLs,
	"email": grabx.Emails,
	"ip":    grabx.IPs,
}

func newGrabCommand(deps grabCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "grab [text|file]",
		Short: "从文本里捞 URL / email / IP（松正则 + stdlib 校验）",
		Long: `从任意文本/日志里把 URL、email、IP 捞出来。松正则找候选 + stdlib 校验留真
（url.Parse / mail.ParseAddress / netip.ParseAddr），所以 999.1.1.1、a@@b 之类会被淘汰。
0 依赖。

例：
  cat access.log | jdan grab -t ip        # 只抽 IP（逐行，可管道）
  jdan grab -t email < contacts.txt       # 只抽邮箱
  pbpaste | jdan grab                      # 抽全部，带类型标签
  jdan grab -t url --count log.txt        # 带出现次数
  jdan grab page.html --json              # 按类型分组 JSON

无参读 stdin；参数是已存在的文件则读文件，否则当作字面文本。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			typeCSV, _ := cmd.Flags().GetString("type")
			withCount, _ := cmd.Flags().GetBool("count")
			doSort, _ := cmd.Flags().GetBool("sort")
			asJSON, _ := cmd.Flags().GetBool("json")

			types, err := parseGrabTypes(typeCSV)
			if err != nil {
				return err
			}
			text, err := readGrabInput(deps.in, args)
			if err != nil {
				return err
			}

			results := make(map[string][]string, len(types))
			for _, t := range types {
				results[t] = grabExtractors[t](text)
			}
			return emitGrab(deps.out, types, results, withCount, doSort, asJSON)
		},
	}
	cmd.Flags().StringP("type", "t", "url,email,ip", "抽取类型（csv）：url / email / ip")
	cmd.Flags().Bool("count", false, "显示每个值的出现次数（不去重统计）")
	cmd.Flags().Bool("sort", false, "结果排序")
	cmd.Flags().Bool("json", false, "按类型分组 JSON 输出")
	return cmd
}

func parseGrabTypes(csv string) ([]string, error) {
	var types []string
	seen := map[string]bool{}
	for raw := range strings.SplitSeq(csv, ",") {
		t := strings.ToLower(strings.TrimSpace(raw))
		switch t {
		case "":
			continue
		case "url", "email", "ip":
			if !seen[t] {
				seen[t] = true
				types = append(types, t)
			}
		default:
			return nil, fmt.Errorf("未知类型 %q（可选 url / email / ip）", t)
		}
	}
	if len(types) == 0 {
		return nil, fmt.Errorf("没有指定有效类型")
	}
	// 按固定顺序排列，输出稳定
	out := make([]string, 0, len(types))
	for _, t := range grabOrder {
		if seen[t] {
			out = append(out, t)
		}
	}
	return out, nil
}

// readGrabInput：无参 → stdin；参数是文件 → 读文件；否则当字面文本。
func readGrabInput(in io.Reader, args []string) (string, error) {
	if len(args) == 0 {
		b, err := io.ReadAll(in)
		return string(b), err
	}
	if fileExists(args[0]) {
		b, err := os.ReadFile(args[0])
		return string(b), err
	}
	return args[0], nil
}

func emitGrab(out io.Writer, types []string, results map[string][]string, withCount, doSort, asJSON bool) error {
	labeled := len(types) > 1

	if asJSON {
		grouped := make(map[string]any, len(types))
		for _, t := range types {
			grouped[t] = grabx.Dedup(results[t])
		}
		return writeIndentJSON(out, grouped)
	}

	for _, t := range types {
		vals := results[t]
		if withCount {
			emitGrabCounts(out, t, vals, labeled, doSort)
			continue
		}
		vals = grabx.Dedup(vals)
		if doSort {
			sort.Strings(vals)
		}
		for _, v := range vals {
			if labeled {
				fmt.Fprintf(out, "%-6s %s\n", t+":", v)
			} else {
				fmt.Fprintln(out, v)
			}
		}
	}
	return nil
}

func emitGrabCounts(out io.Writer, t string, vals []string, labeled, doSort bool) {
	counts := map[string]int{}
	var order []string
	for _, v := range vals {
		if counts[v] == 0 {
			order = append(order, v)
		}
		counts[v]++
	}
	if doSort {
		sort.Strings(order)
	} else {
		// 默认按出现次数降序
		sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	}
	for _, v := range order {
		if labeled {
			fmt.Fprintf(out, "%-6s %5d  %s\n", t+":", counts[v], v)
		} else {
			fmt.Fprintf(out, "%5d  %s\n", counts[v], v)
		}
	}
}

func init() {
	rootCmd.AddCommand(newGrabCommand(grabCmdDeps{}))
}
