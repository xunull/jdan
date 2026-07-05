package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/netip"
	"os"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/ipx"
)

type ipCmdDeps struct {
	out io.Writer
}

// ipCmdExitErr 是 contains 报告 "not contained" 的退出码错误（exit 1，不带 noise）。
type ipCmdExitErr struct{ msg string }

func (e *ipCmdExitErr) Error() string { return e.msg }

func newIPCommand(deps ipCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "ip",
		Short: "IP 地址 & CIDR 工具集（info/contains/range/split/normalize）",
		Long: `IP / CIDR 计算工具。0 新依赖，全 stdlib net/netip。

子命令：
  info        综合信息（吃 IP 或 CIDR）
  contains    判断 IP 是否在 CIDR 内（退出码 0/1，CI gate 用）
  range       列出 CIDR 内的 IP（前 N 个）
  range-cidr  任意起止 IP 区间 → 最小 CIDR 集
  split       子网划分（10.0.0.0/22 split 24 → 4 个 /24）
  aggregate   合并一组网段为最小 CIDR 覆盖集（split 的逆运算）
  normalize   IPv6 标准化（expand / compact）

例：
  jdan ip info 192.168.1.0/24
  jdan ip contains 10.0.0.0/8 10.5.1.2 && echo "internal"
  jdan ip range 192.168.1.0/29
  jdan ip range-cidr 192.168.1.5 192.168.1.20
  jdan ip split 10.0.0.0/22 24
  jdan ip aggregate 10.0.0.0/25 10.0.0.128/25
  jdan ip normalize 2001:db8::1 --expand`,
	}
	cmd.AddCommand(newIPInfoCommand(deps))
	cmd.AddCommand(newIPContainsCommand(deps))
	cmd.AddCommand(newIPRangeCommand(deps))
	cmd.AddCommand(newIPRangeCIDRCommand(deps))
	cmd.AddCommand(newIPSplitCommand(deps))
	cmd.AddCommand(newIPAggregateCommand(deps))
	cmd.AddCommand(newIPNormalizeCommand(deps))
	return cmd
}

// ---- info ----

func newIPInfoCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "info <ip|cidr>",
		Short:         "综合信息（吃 IP 或 CIDR）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runIPInfo(args[0], asJSON, deps.out)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runIPInfo(target string, asJSON bool, out io.Writer) error {
	// 先尝试 CIDR，再 fallback single addr
	if p, err := netip.ParsePrefix(target); err == nil {
		info, err := ipx.ComputeCIDR(p)
		if err != nil {
			return err
		}
		if asJSON {
			return jsonEncode(out, info)
		}
		renderCIDRInfo(out, info)
		return nil
	}
	addr, err := netip.ParseAddr(target)
	if err != nil {
		return fmt.Errorf("invalid IP or CIDR: %q", target)
	}
	info := ipx.ComputeAddrInfo(addr)
	if asJSON {
		return jsonEncode(out, info)
	}
	renderAddrInfo(out, info)
	return nil
}

func renderAddrInfo(w io.Writer, info ipx.AddrInfo) {
	row := func(label, val string) {
		if val != "" {
			fmt.Fprintf(w, "  %-15s %s\n", label+":", val)
		}
	}
	row("Address", info.Address)
	row("Version", fmt.Sprintf("IPv%d", info.Version))
	if info.Version == 6 {
		row("Compact", info.Compact)
		row("Expanded", info.Expanded)
	}
	row("Hex", info.Hex)
	row("Decimal", info.Decimal)
	row("Binary", info.Binary)
	row("Reverse DNS", info.ReverseDNS)
	fmt.Fprintln(w, "\n  Classification:")
	c := info.Classification
	yes := func(label string, b bool) {
		if b {
			fmt.Fprintf(w, "  %-15s yes\n", label+":")
		}
	}
	yes("Private", c.Private)
	yes("Loopback", c.Loopback)
	yes("Multicast", c.Multicast)
	yes("Link-local", c.LinkLocal)
	yes("Unspecified", c.Unspecified)
	yes("Doc range", c.Documentation)
	yes("Unique local", c.UniqueLocal)
	yes("CGNAT", c.CGNAT)
	yes("Global unicast", c.GlobalUnicast)
}

func renderCIDRInfo(w io.Writer, info ipx.CIDRInfo) {
	row := func(label, val string) {
		fmt.Fprintf(w, "  %-15s %s\n", label+":", val)
	}
	row("CIDR", info.Prefix.String())
	addr := info.Prefix.Addr()
	if addr.Is4() {
		row("Version", "IPv4")
		row("Network", info.Network.String())
		row("Broadcast", info.Broadcast.String())
		row("First host", info.FirstHost.String())
		row("Last host", info.LastHost.String())
		row("Netmask", info.Netmask.String())
		row("Wildcard", info.Wildcard.String())
		row("Total IPs", info.TotalAddrs.String())
		row("Usable", info.UsableAddrs.String())
	} else {
		row("Version", "IPv6")
		row("Network", info.Network.String())
		row("First host", info.FirstHost.String())
		row("Last host", info.LastHost.String())
		row("Prefix len", fmt.Sprintf("%d", info.PrefixLen))
		row("Total IPs", info.TotalAddrs.String())
	}
}

// ---- contains ----

func newIPContainsCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "contains <cidr> <ip>",
		Short:         "判断 IP 是否在 CIDR 内（退出码 0/1）",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			verbose, _ := cmd.Flags().GetBool("verbose")
			return runIPContains(args[0], args[1], verbose, deps.out)
		},
	}
	cmd.Flags().BoolP("verbose", "v", false, "输出 'yes' / 'no'（默认只用退出码）")
	return cmd
}

func runIPContains(cidrStr, ipStr string, verbose bool, out io.Writer) error {
	p, err := netip.ParsePrefix(cidrStr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %q", cidrStr)
	}
	addr, err := netip.ParseAddr(ipStr)
	if err != nil {
		return fmt.Errorf("invalid IP: %q", ipStr)
	}
	// 类型必须匹配：IPv4 CIDR 不能含 IPv6
	if p.Addr().Is4() != addr.Is4() {
		return fmt.Errorf("address family mismatch: %s is %s, %s is %s",
			cidrStr, ipFamily(p.Addr()), ipStr, ipFamily(addr))
	}
	contained := p.Contains(addr)
	if verbose {
		if contained {
			fmt.Fprintln(out, "yes")
		} else {
			fmt.Fprintln(out, "no")
		}
	}
	if !contained {
		return &ipCmdExitErr{msg: fmt.Sprintf("%s not in %s", ipStr, cidrStr)}
	}
	return nil
}

func ipFamily(addr netip.Addr) string {
	if addr.Is4() {
		return "IPv4"
	}
	return "IPv6"
}

// ---- range ----

func newIPRangeCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "range <cidr>",
		Short:         "列出 CIDR 内的 IP（默认前 16 个）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			asJSON, _ := cmd.Flags().GetBool("json")
			return runIPRange(args[0], limit, asJSON, deps.out)
		},
	}
	cmd.Flags().Int("limit", 16, "列出前 N 个（0 = 全列，硬上限 16M 防 OOM）")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runIPRange(cidrStr string, limit int, asJSON bool, out io.Writer) error {
	p, err := netip.ParsePrefix(cidrStr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %q", cidrStr)
	}
	res, err := ipx.Range(p, limit)
	if err != nil {
		return err
	}
	if asJSON {
		addrs := make([]string, len(res.Addrs))
		for i, a := range res.Addrs {
			addrs[i] = a.String()
		}
		return jsonEncode(out, map[string]any{
			"cidr":     cidrStr,
			"returned": res.Returned,
			"total":    res.Total.String(),
			"addrs":    addrs,
		})
	}
	for _, a := range res.Addrs {
		fmt.Fprintln(out, a.String())
	}
	// summary 行
	if res.Total.Cmp(big.NewInt(int64(res.Returned))) > 0 {
		fmt.Fprintf(out, "... (%s total, showing first %d; use --limit N or --limit 0 for all)\n",
			res.Total.String(), res.Returned)
	} else {
		fmt.Fprintf(out, "(%d total)\n", res.Returned)
	}
	return nil
}

// ---- split ----

func newIPSplitCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "split <cidr> <new-prefix-bits>",
		Short:         "子网划分（10.0.0.0/22 split 24 → 4 个 /24）",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			return runIPSplit(args[0], args[1], asJSON, deps.out)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runIPSplit(cidrStr, newBitsStr string, asJSON bool, out io.Writer) error {
	p, err := netip.ParsePrefix(cidrStr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %q", cidrStr)
	}
	newBits, err := parsePositiveInt(newBitsStr)
	if err != nil {
		return fmt.Errorf("invalid new prefix bits: %q", newBitsStr)
	}
	subnets, err := ipx.Split(p, newBits)
	if err != nil {
		return err
	}
	if asJSON {
		strs := make([]string, len(subnets))
		for i, s := range subnets {
			strs[i] = s.String()
		}
		return jsonEncode(out, map[string]any{
			"parent":  cidrStr,
			"new_len": newBits,
			"count":   len(subnets),
			"subnets": strs,
		})
	}
	for _, s := range subnets {
		fmt.Fprintln(out, s.String())
	}
	fmt.Fprintf(out, "(%d subnets)\n", len(subnets))
	return nil
}

// ---- normalize ----

func newIPNormalizeCommand(deps ipCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "normalize <ipv6>",
		Short:         "IPv6 标准化（expand / compact）",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			expand, _ := cmd.Flags().GetBool("expand")
			compact, _ := cmd.Flags().GetBool("compact")
			if expand && compact {
				return fmt.Errorf("--expand 和 --compact 互斥")
			}
			return runIPNormalize(args[0], expand, deps.out)
		},
	}
	cmd.Flags().Bool("expand", false, "完整 8 段输出（IPv6 only）")
	cmd.Flags().Bool("compact", false, "compact 形式（IPv6 默认就是 compact）")
	return cmd
}

func runIPNormalize(target string, expand bool, out io.Writer) error {
	addr, err := netip.ParseAddr(target)
	if err != nil {
		return fmt.Errorf("invalid IP: %q", target)
	}
	s, err := ipx.Normalize(addr, expand)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, s)
	return nil
}

// ---- helpers ----

func jsonEncode(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a positive integer: %q", s)
		}
		n = n*10 + int(c-'0')
		if n > 128 {
			return 0, fmt.Errorf("value too large: %q", s)
		}
	}
	if n == 0 && s != "0" {
		return 0, fmt.Errorf("empty string")
	}
	return n, nil
}

func init() {
	rootCmd.AddCommand(newIPCommand(ipCmdDeps{}))
}
