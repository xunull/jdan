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

	"github.com/xunull/jdan/internal/sslcert"
	"github.com/xunull/jdan/internal/sslscan"
)

type sslScanCmdDeps struct {
	out io.Writer
}

type sslScanExitErr struct{ msg string }

func (e *sslScanExitErr) Error() string { return e.msg }

func newSSLScanCommand(deps sslScanCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "scan <host[:port]>",
		Short: "TLS 配置综合审计 (本地 ssllabs 替代)",
		Long: `综合审计一个 HTTPS host 的 TLS 配置：
  - TLS 版本支持 (1.0/1.1/1.2/1.3)
  - Cipher Suites (TLS 1.2)
  - ALPN 协议 (h2 / http/1.1)
  - HSTS 强度
  - Session resumption
  - Cert 概要

最后给出 ssllabs 风格的 A+/A/B/C/D/F 评分。

例：
  jdan ssl scan github.com
  jdan ssl scan example.com:8443
  jdan ssl scan internal.example --sni public.example
  jdan ssl scan example.com --json
  jdan ssl scan example.com --full-cipher   # 试 ~40 个 cipher 而不是 16 个
  jdan ssl scan example.com --grade-only    # 只输出 grade；C 以下 exit 1`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSLScan(cmd, args[0], deps.out)
		},
	}
	cmd.Flags().String("sni", "", "TLS SNI 名（默认用 host）")
	cmd.Flags().Bool("full-cipher", false, "试 40+ 个 cipher 而不是 16 个常见")
	cmd.Flags().Bool("no-cipher", false, "跳过 cipher 枚举（更快）")
	cmd.Flags().Bool("no-hsts", false, "跳过 HSTS 检测")
	cmd.Flags().Bool("no-resume", false, "跳过 session resumption 检测")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().Bool("grade-only", false, "只输出 grade 字母；C 以下 exit 1（CI/CD 监控）")
	cmd.Flags().Duration("timeout", 15*time.Second, "整体超时")
	return cmd
}

func runSSLScan(cmd *cobra.Command, target string, out io.Writer) error {
	sni, _ := cmd.Flags().GetString("sni")
	fullCipher, _ := cmd.Flags().GetBool("full-cipher")
	noCipher, _ := cmd.Flags().GetBool("no-cipher")
	noHSTS, _ := cmd.Flags().GetBool("no-hsts")
	noResume, _ := cmd.Flags().GetBool("no-resume")
	asJSON, _ := cmd.Flags().GetBool("json")
	gradeOnly, _ := cmd.Flags().GetBool("grade-only")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	host, port, err := sslcert.ParseTarget(target)
	if err != nil {
		return err
	}

	report, err := sslscan.Scan(context.Background(), sslscan.Options{
		Host:       host,
		Port:       port,
		SNI:        sni,
		Timeout:    timeout,
		FullCipher: fullCipher,
		SkipCipher: noCipher,
		SkipHSTS:   noHSTS,
		SkipResume: noResume,
	})
	if err != nil {
		return err
	}

	switch {
	case gradeOnly:
		fmt.Fprintln(out, report.Grade.Letter)
		if report.Grade.IsFailing() {
			return &sslScanExitErr{
				msg: fmt.Sprintf("grade %s below threshold B", report.Grade.Letter),
			}
		}
		return nil
	case asJSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	renderScanText(out, report)
	return nil
}

func renderScanText(out io.Writer, r *sslscan.ScanReport) {
	// ─── TLS 版本 box ───
	renderTLSVersionsBox(out, r.Versions)
	fmt.Fprintln(out)

	// ─── Cipher suites box ───
	if len(r.Ciphers.TLS12) > 0 {
		renderCiphersBox(out, r.Ciphers)
		fmt.Fprintln(out)
	}

	// ─── HTTP stack box (ALPN + HSTS) ───
	renderHTTPStackBox(out, r.ALPN, r.HSTS)
	fmt.Fprintln(out)

	// ─── Cert box ───
	if r.Cert != nil {
		renderCertScanBox(out, r.Cert)
		fmt.Fprintln(out)
	}

	// ─── Session resumption ───
	renderResumeLine(out, r.Resume)
	fmt.Fprintln(out)

	// ─── Grade ───
	renderGradeBlock(out, r.Grade)

	// 总耗时
	fmt.Fprintf(out, "\nscan time: %s\n", formatMs(r.Elapsed))
}

const scanBoxWidth = 60

func renderTLSVersionsBox(out io.Writer, v sslscan.VersionsSection) {
	header := "TLS Versions"
	printBoxOpen(out, header)
	for _, r := range v.Results {
		icon := "✗"
		note := ""
		if r.Supported {
			icon = "✓"
			if r.Version == "TLS 1.3" {
				note = " (preferred)"
			}
			if r.Deprecated {
				note = " (DEPRECATED, weak)"
			}
		} else if r.Deprecated {
			note = " (recommended off)"
		}
		printBoxRow(out, fmt.Sprintf("%s %-7s   %s%s", icon, r.Version, supportedText(r.Supported), note))
	}
	printBoxClose(out)
}

func supportedText(b bool) string {
	if b {
		return "supported"
	}
	return "refused   "
}

func renderCiphersBox(out io.Writer, c sslscan.CiphersSection) {
	printBoxOpen(out, "Cipher Suites (TLS 1.2)")
	for _, r := range c.TLS12 {
		if !r.Supported {
			continue
		}
		icon := "✓"
		tag := "(strong)"
		switch r.Strength {
		case sslscan.StrengthAcceptable:
			tag = "(acceptable; no forward sec)"
		case sslscan.StrengthWeak:
			icon = "✗"
			tag = "(WEAK)"
		}
		name := truncateMiddle(r.Name, 40)
		printBoxRow(out, fmt.Sprintf("%s %-40s %s", icon, name, tag))
	}
	// 显示拒绝的弱密（让用户能验证 server 拒了）
	hasShownWeakRefused := false
	for _, r := range c.TLS12 {
		if r.Supported || r.Strength != sslscan.StrengthWeak {
			continue
		}
		if !hasShownWeakRefused {
			printBoxRow(out, "")
			printBoxRow(out, "Weak ciphers correctly refused:")
			hasShownWeakRefused = true
		}
		printBoxRow(out, fmt.Sprintf("  ✓ %s refused", truncateMiddle(r.Name, 40)))
	}
	if c.TLS13Note != "" {
		printBoxRow(out, "")
		printBoxRow(out, c.TLS13Note)
	}
	printBoxClose(out)
}

func renderHTTPStackBox(out io.Writer, a sslscan.ALPNSection, h *sslscan.HSTSSection) {
	printBoxOpen(out, "HTTP Stack")
	// ALPN
	alpnList := strings.Join(a.Supported, ", ")
	if alpnList == "" {
		alpnList = "none"
	}
	printBoxRow(out, fmt.Sprintf("ALPN:    %s", alpnList))
	// HSTS
	if h == nil || !h.Present {
		printBoxRow(out, "HSTS:    not configured")
	} else {
		printBoxRow(out, fmt.Sprintf("HSTS:    %s", h.RawHeader))
		printBoxRow(out, fmt.Sprintf("         strength=%s, max-age=%d", h.Strength(), h.MaxAge))
	}
	printBoxClose(out)
}

func renderCertScanBox(out io.Writer, c *sslscan.CertSection) {
	printBoxOpen(out, "Cert")
	printBoxRow(out, fmt.Sprintf("Subject:    CN=%s", c.SubjectCN))
	printBoxRow(out, fmt.Sprintf("Issuer:     CN=%s", truncateMiddle(c.IssuerCN, 40)))
	printBoxRow(out, fmt.Sprintf("Key:        %s", c.KeyAlgorithm))
	printBoxRow(out, fmt.Sprintf("Signed:     %s", c.SigAlgorithm))
	printBoxRow(out, fmt.Sprintf("Days left:  %d", c.DaysLeft))

	chainStatus := "trusted ✓"
	if !c.Trusted {
		chainStatus = "NOT trusted ✗"
	}
	hostStatus := "matches SAN ✓"
	if !c.HostnameOK {
		hostStatus = "MISMATCH ✗"
	}
	printBoxRow(out, fmt.Sprintf("Chain:      %s", chainStatus))
	printBoxRow(out, fmt.Sprintf("Hostname:   %s", hostStatus))
	printBoxClose(out)
}

func renderResumeLine(out io.Writer, r sslscan.ResumeSection) {
	tls12 := "✗ refused"
	if r.TLS12TicketSupported {
		tls12 = "✓ supported"
	}
	tls13 := "✗ refused"
	if r.TLS13PSKSupported {
		tls13 = "✓ supported"
	}
	fmt.Fprintf(out, "Session Resumption:\n")
	fmt.Fprintf(out, "  TLS 1.2 session ticket: %s\n", tls12)
	fmt.Fprintf(out, "  TLS 1.3 PSK:            %s\n", tls13)
}

func renderGradeBlock(out io.Writer, g sslscan.GradeReport) {
	fmt.Fprintf(out, "Overall: %s  (%d/100)\n", g.Letter, g.Score)
	if len(g.Strengths) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Strong points:")
		for _, s := range g.Strengths {
			fmt.Fprintf(out, "  ✓ %s\n", s)
		}
	}
	if len(g.Concerns) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Concerns:")
		for _, c := range g.Concerns {
			fmt.Fprintf(out, "  ⚠ %s\n", c)
		}
	}
}

// box drawing helpers（与 ssl_cert_render 风格保持一致）
func printBoxOpen(out io.Writer, title string) {
	dashes := strings.Repeat("─", scanBoxWidth-3-len(title))
	fmt.Fprintf(out, "╭─ %s %s╮\n", title, dashes)
}

func printBoxRow(out io.Writer, content string) {
	if visibleLen(content) > scanBoxWidth-4 {
		content = truncateMiddle(content, scanBoxWidth-4)
	}
	padded := padRight(content, scanBoxWidth-4)
	fmt.Fprintf(out, "│ %s │\n", padded)
}

func printBoxClose(out io.Writer) {
	fmt.Fprintln(out, "╰"+strings.Repeat("─", scanBoxWidth-2)+"╯")
}

func init() {
	sslCmd.AddCommand(newSSLScanCommand(sslScanCmdDeps{}))
}
