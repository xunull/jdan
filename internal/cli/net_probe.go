package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/netprobe"
)

type netProbeCmdDeps struct {
	out io.Writer
}

func newNetProbeCommand(deps netProbeCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "probe <target>",
		Short: "客户端视角探查目标主机/端口/URL",
		Long: `从客户端视角逐阶段探查目标，逐阶段实时输出 DNS / TCP / TLS / HTTP 的结果。
失败时给出针对性的修复 hint，引导下一步排查。

例：
  jdan net probe https://github.com
  jdan net probe example.com                # 默认 https
  jdan net probe example.com:80             # 显式 http 端口
  jdan net probe 192.168.1.42:8080          # IP literal
  jdan net probe github.com --resolver 8.8.8.8
  jdan net probe https://self-signed.local --insecure
  jdan net probe github.com --json          # 脚本消费
  jdan net probe github.com --verbose       # cert chain + all headers
  jdan net probe github.com --method GET    # 默认 HEAD`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true, // 自己渲染错误，不让 zerolog FTL 接管
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNetProbe(cmd, args[0], deps.out)
		},
	}
	cmd.Flags().Duration("timeout", 0, "单个阶段超时（默认 10s）")
	cmd.Flags().String("resolver", "", "DNS 服务器（host[:port]），空 = 系统默认")
	cmd.Flags().String("method", "HEAD", "HTTP 方法；405 时自动 fallback GET")
	cmd.Flags().BoolP("insecure", "k", false, "跳过 TLS 证书验证")
	cmd.Flags().BoolP("verbose", "v", false, "显示完整 cert chain + 所有响应 header")
	cmd.Flags().Bool("json", false, "输出 JSON 而不是 text")
	return cmd
}

func runNetProbe(cmd *cobra.Command, target string, out io.Writer) error {
	timeout, _ := cmd.Flags().GetDuration("timeout")
	resolver, _ := cmd.Flags().GetString("resolver")
	method, _ := cmd.Flags().GetString("method")
	insecure, _ := cmd.Flags().GetBool("insecure")
	verbose, _ := cmd.Flags().GetBool("verbose")
	asJSON, _ := cmd.Flags().GetBool("json")

	opts := netprobe.Options{
		Timeout:  timeout,
		Resolver: resolver,
		Method:   method,
		Insecure: insecure,
		Verbose:  verbose,
	}

	var emit func(*netprobe.StageResult)
	if !asJSON {
		emit = func(s *netprobe.StageResult) {
			renderStage(out, s, verbose)
		}
	}

	res, err := netprobe.Probe(context.Background(), target, opts, emit)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	// 总结
	fmt.Fprintln(out)
	if res.OK {
		fmt.Fprintf(out, "✓ all green · total %s\n", formatMs(res.Total))
	} else {
		fmt.Fprintf(out, "✗ failed at %s · total %s\n", res.Stopped, formatMs(res.Total))
	}
	// probe 完成本身视为命令成功——失败的诊断信息已经在 stages 里全显示。
	// 脚本想识别 probe 是否通用 --json 看 .ok 字段。
	return nil
}

func renderStage(out io.Writer, s *netprobe.StageResult, verbose bool) {
	icon := "◇"
	if s.Success {
		icon = "✓"
	} else {
		icon = "✗"
	}

	switch s.Stage {
	case netprobe.StageResolve:
		fmt.Fprintf(out, "%s resolve     %s\n", icon, s.Detail)
		if s.Resolve != nil && len(s.Resolve.IPs) > 1 {
			for _, ip := range s.Resolve.IPs {
				kind := "A"
				if ip.To4() == nil {
					kind = "AAAA"
				}
				fmt.Fprintf(out, "  → %s (%s)\n", ip, kind)
			}
		}
		fmt.Fprintf(out, "  duration: %s\n", formatMs(s.Duration))

	case netprobe.StageTCP:
		fmt.Fprintf(out, "%s tcp         %s\n", icon, s.Detail)
		if s.TCP != nil {
			for _, a := range s.TCP.Attempts {
				if a.Success {
					fmt.Fprintf(out, "  ✓ %s from %s (%s)\n", a.IP, a.LocalAddr, formatMs(a.Duration))
				} else {
					fmt.Fprintf(out, "  ✗ %s: %s\n", a.IP, a.Err)
				}
			}
		}

	case netprobe.StageTLS:
		if s.Success && s.TLS != nil {
			fmt.Fprintf(out, "%s tls         %s\n", icon, s.Detail)
			fmt.Fprintf(out, "  ALPN=%s, SNI=%s, duration=%s\n", s.TLS.ALPN, s.TLS.SNI, formatMs(s.Duration))
			if verbose {
				fmt.Fprintf(out, "  cipher: %s\n", s.TLS.CipherSuite)
				fmt.Fprintf(out, "  chain depth: %d\n", s.TLS.ChainDepth)
			}
		} else {
			fmt.Fprintf(out, "%s tls         (%s)\n", icon, formatMs(s.Duration))
		}

	case netprobe.StageHTTP:
		fmt.Fprintf(out, "%s http        %s\n", icon, s.Detail)
		if s.HTTP != nil {
			if s.HTTP.Server != "" {
				fmt.Fprintf(out, "  server: %s\n", s.HTTP.Server)
			}
			if s.HTTP.ContentLength > 0 {
				fmt.Fprintf(out, "  content-length: %d\n", s.HTTP.ContentLength)
			}
			if s.HTTP.FellBackToGET {
				fmt.Fprintln(out, "  (HEAD returned 405; fell back to GET)")
			}
			if verbose && len(s.HTTP.Headers) > 0 {
				fmt.Fprintln(out, "  headers:")
				for k, v := range s.HTTP.Headers {
					fmt.Fprintf(out, "    %s: %s\n", k, v)
				}
			}
		}
		fmt.Fprintf(out, "  duration: %s\n", formatMs(s.Duration))
	}

	if !s.Success && s.Err != "" {
		fmt.Fprintf(out, "  error: %s\n", s.Err)
	}
	if s.Hint != "" {
		fmt.Fprintln(out)
		for _, line := range strings.Split(s.Hint, "\n") {
			fmt.Fprintf(out, "  %s\n", line)
		}
	}
	fmt.Fprintln(out)
}

func formatMs(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func init() {
	netCmd.AddCommand(newNetProbeCommand(netProbeCmdDeps{}))
}
