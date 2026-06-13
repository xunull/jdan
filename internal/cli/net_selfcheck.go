package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/netcheck"
)

type netSelfcheckCmdDeps struct {
	out io.Writer
}

func newNetSelfcheckCommand(deps netSelfcheckCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "selfcheck [:port]",
		Short: "服务端自检：firewall + 接口 + 监听端口 + 自连接 + 预测",
		Long: `服务端视角的诊断："我作为 server 该不该被外部访问？"

输入可选端口号（带或不带冒号）。指定端口后会用 lsof 看谁在监听、bind 地址
是不是 LAN-reachable，再用 HTTP GET 自己访问 localhost 和 LAN IP 看通不通，
最后综合给一句"外部客户端应当能/不能访问"的预测。

例：
  jdan net selfcheck              # 通用诊断（OS / firewall / 接口）
  jdan net selfcheck 8080         # +查 :8080 监听情况
  jdan net selfcheck :8080        # 同上（冒号可有可无）
  jdan net selfcheck 8080 --json  # 结构化输出`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNetSelfcheck(cmd, args, deps.out)
		},
	}
	cmd.Flags().Bool("json", false, "输出 JSON 而不是 text")
	return cmd
}

func runNetSelfcheck(cmd *cobra.Command, args []string, out io.Writer) error {
	asJSON, _ := cmd.Flags().GetBool("json")

	opts := netcheck.Options{}
	if len(args) > 0 {
		port, err := parsePortArg(args[0])
		if err != nil {
			return err
		}
		opts.Port = port
	}

	report, err := netcheck.SelfCheck(context.Background(), opts)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	renderReport(out, report)
	return nil
}

func parsePortArg(s string) (int, error) {
	s = strings.TrimPrefix(s, ":")
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid port %q", s)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("port %d out of range", port)
	}
	return port, nil
}

func renderReport(out io.Writer, r *netcheck.Report) {
	// OS + firewall
	fmt.Fprintln(out, "◇ os & firewall")
	fmt.Fprintf(out, "  • %s/%s\n", r.OS, r.Arch)
	switch r.Firewall.State {
	case "enabled":
		fmt.Fprintln(out, "  ⚠ Application Firewall: ON")
	case "disabled":
		fmt.Fprintln(out, "  ✓ Application Firewall: off")
	default:
		fmt.Fprintf(out, "  ? Application Firewall: unknown\n")
	}
	if r.Firewall.Hint != "" {
		for _, line := range strings.Split(r.Firewall.Hint, "\n") {
			fmt.Fprintf(out, "    %s\n", line)
		}
	}
	fmt.Fprintln(out)

	// interfaces
	fmt.Fprintln(out, "◇ network interfaces")
	for _, iface := range r.Interfaces {
		mark := " "
		if iface.Primary {
			mark = "★"
		}
		flags := []string{}
		if iface.Primary {
			flags = append(flags, "primary")
		}
		if iface.LAN {
			flags = append(flags, "LAN")
		}
		if iface.Loopback {
			flags = append(flags, "loopback")
		}
		flagStr := ""
		if len(flags) > 0 {
			flagStr = " (" + strings.Join(flags, ", ") + ")"
		}
		fmt.Fprintf(out, "  %s %s%s\n", mark, iface.Name, flagStr)
		for _, ip := range iface.IPs {
			fmt.Fprintf(out, "      %s\n", ip)
		}
	}
	fmt.Fprintln(out)

	// listening (optional)
	if r.Listening != nil {
		fmt.Fprintf(out, "◇ listening on :%d\n", r.Listening.Port)
		if r.Listening.LsofMissing {
			fmt.Fprintln(out, "  ⚠ can't determine: lsof not installed")
			fmt.Fprintln(out, "    install: brew install lsof  /  apt install lsof")
		} else if len(r.Listening.Listeners) == 0 {
			fmt.Fprintln(out, "  ✗ no listener on this port")
		} else {
			for _, l := range r.Listening.Listeners {
				reach := "LAN-reachable"
				if !l.IsLANReachable() {
					reach = "localhost-only"
				}
				fmt.Fprintf(out, "  ✓ %s (pid %d, user %s) bind=%s (%s)\n",
					l.Process, l.PID, l.User, l.Bind, reach)
			}
		}
		fmt.Fprintln(out)
	}

	// self-loop (optional)
	if r.SelfLoop != nil {
		fmt.Fprintln(out, "◇ self-loop test")
		printConnect(out, r.SelfLoop.Localhost)
		if r.SelfLoop.PrimaryLAN.Addr != "" {
			printConnect(out, r.SelfLoop.PrimaryLAN)
		}
		fmt.Fprintln(out)
	}

	// prediction
	fmt.Fprintln(out, "◇ prediction")
	for _, line := range strings.Split(r.Prediction, "\n") {
		fmt.Fprintf(out, "  %s\n", line)
	}
}

func printConnect(out io.Writer, c netcheck.ConnectResult) {
	if c.Success {
		fmt.Fprintf(out, "  ✓ %s in %s\n", c.Addr, formatMs(c.Duration))
	} else {
		fmt.Fprintf(out, "  ✗ %s: %s\n", c.Addr, c.Err)
	}
}

func init() {
	netCmd.AddCommand(newNetSelfcheckCommand(netSelfcheckCmdDeps{}))
}
