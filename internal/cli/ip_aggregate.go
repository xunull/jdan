package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/ipx"
)

// ---- aggregate ----

func newIPAggregateCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aggregate [cidr|ip ...]",
		Short: "合并一组网段为最小 CIDR 覆盖集（split 的逆运算）",
		Long: `把一组 CIDR / IP 合并成最小的 CIDR 覆盖集：重叠或相邻的网段被并起来。
IPv4 与 IPv6 各自聚合。参数留空时从 stdin 读（空白/换行分隔）。

例：
  jdan ip aggregate 10.0.0.0/25 10.0.0.128/25      # → 10.0.0.0/24
  jdan ip aggregate 10.0.0.0/24 10.0.0.128/25      # 被包含 → 10.0.0.0/24
  cat routes.txt | jdan ip aggregate               # 从 stdin
  jdan ip aggregate 10.0.0.0/25 10.0.0.128/25 --json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runIPAggregate(args, cmd.InOrStdin(), asJSON, deps.out)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runIPAggregate(args []string, in io.Reader, asJSON bool, out io.Writer) error {
	tokens := args
	if len(tokens) == 0 {
		tokens = readTokens(in)
	}
	if len(tokens) == 0 {
		return fmt.Errorf("没有输入网段（给参数或走 stdin）")
	}
	prefixes := make([]netip.Prefix, 0, len(tokens))
	for _, tok := range tokens {
		p, err := parsePrefixOrAddr(tok)
		if err != nil {
			return err
		}
		prefixes = append(prefixes, p)
	}
	result, err := ipx.Aggregate(prefixes)
	if err != nil {
		return err
	}
	if asJSON {
		strs := make([]string, len(result))
		for i, p := range result {
			strs[i] = p.String()
		}
		return jsonEncode(out, map[string]any{
			"in":    len(prefixes),
			"out":   len(result),
			"cidrs": strs,
		})
	}
	for _, p := range result {
		fmt.Fprintln(out, p.String())
	}
	fmt.Fprintf(out, "(%d in → %d out)\n", len(prefixes), len(result))
	return nil
}

// ---- range-cidr ----

func newIPRangeCIDRCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "range-cidr <start> <end>",
		Short: "任意起止 IP 区间 → 最小 CIDR 集（range 的反向）",
		Long: `把任意起止 IP 区间（不必对齐边界）分解成最小数量的 CIDR。
可写成两个参数，或单个 "start-end"。

例：
  jdan ip range-cidr 192.168.1.5 192.168.1.20
  jdan ip range-cidr 192.168.1.5-192.168.1.20     # 单参数写法
  jdan ip range-cidr 2001:db8:: 2001:db8::ff
  jdan ip range-cidr 10.0.0.5 10.0.0.20 --json`,
		Args:          cobra.RangeArgs(1, 2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			startStr, endStr, err := splitRangeArgs(args)
			if err != nil {
				return err
			}
			return runIPRangeCIDR(startStr, endStr, asJSON, deps.out)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runIPRangeCIDR(startStr, endStr string, asJSON bool, out io.Writer) error {
	start, err := netip.ParseAddr(startStr)
	if err != nil {
		return fmt.Errorf("invalid start IP: %q", startStr)
	}
	end, err := netip.ParseAddr(endStr)
	if err != nil {
		return fmt.Errorf("invalid end IP: %q", endStr)
	}
	if start.Is4() != end.Is4() {
		return fmt.Errorf("address family mismatch: %s is %s, %s is %s",
			startStr, ipFamily(start), endStr, ipFamily(end))
	}
	cidrs, err := ipx.RangeToCIDRs(start, end)
	if err != nil {
		return err
	}
	if asJSON {
		strs := make([]string, len(cidrs))
		for i, p := range cidrs {
			strs[i] = p.String()
		}
		return jsonEncode(out, map[string]any{
			"start": start.String(),
			"end":   end.String(),
			"count": len(cidrs),
			"cidrs": strs,
		})
	}
	for _, p := range cidrs {
		fmt.Fprintln(out, p.String())
	}
	fmt.Fprintf(out, "(%d CIDRs)\n", len(cidrs))
	return nil
}

// ---- helpers ----

// parsePrefixOrAddr 把一个 token 解析成 prefix：CIDR 原样（Masked），裸 IP → /32 或 /128。
func parsePrefixOrAddr(s string) (netip.Prefix, error) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Masked(), nil
	}
	if a, err := netip.ParseAddr(s); err == nil {
		a = a.Unmap()
		return a.Prefix(a.BitLen())
	}
	return netip.Prefix{}, fmt.Errorf("invalid IP or CIDR: %q", s)
}

// splitRangeArgs 支持两参数或单个 "start-end"。IPv4/IPv6 地址本身都不含 '-'，
// 故单参数按第一个 '-' 切开是安全的。
func splitRangeArgs(args []string) (string, string, error) {
	if len(args) == 2 {
		return args[0], args[1], nil
	}
	one := strings.TrimSpace(args[0])
	i := strings.IndexByte(one, '-')
	if i <= 0 || i >= len(one)-1 {
		return "", "", fmt.Errorf("单参数需写成 start-end：%q", args[0])
	}
	return one[:i], one[i+1:], nil
}

// readTokens 从 reader 读所有空白/换行分隔的 token。
func readTokens(in io.Reader) []string {
	if in == nil {
		in = os.Stdin
	}
	var tokens []string
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			tokens = append(tokens, t)
		}
	}
	return tokens
}
