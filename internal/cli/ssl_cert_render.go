package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/xunull/jdan/internal/sslcert"
)

// renderCertText 渲染 jdan ssl cert 的人类视角输出：
// box 包裹的 leaf summary + chain 列表 + verification + OCSP。
func renderCertText(out io.Writer, b *sslcert.Bundle, report *sslcert.VerificationReport, ocsp []sslcert.OCSPStatus, full bool) {
	leafSummary := sslcert.Describe(b.Leaf())

	// box: leaf
	renderLeafBox(out, leafSummary, full)

	// chain
	chain := b.FullChain()
	if len(chain) > 1 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Chain:")
		for i, c := range chain {
			role := "leaf"
			switch i {
			case len(chain) - 1:
				if c.Subject.String() == c.Issuer.String() {
					role = "root"
				} else {
					role = "intermediate"
				}
			case 0:
				role = "leaf"
			default:
				role = "intermediate"
			}
			s := sslcert.Describe(c)
			extra := ""
			if role == "root" && c.Subject.String() == c.Issuer.String() {
				extra = ", self-signed"
			}
			if i == 0 && b.RootFromTrust() != nil && i == len(b.Chain) && len(b.Chain) < len(chain) {
				extra += ", from system trust"
			}
			fmt.Fprintf(out, "  ▸ %-12s %s  (exp in %dd%s)\n", role+":", sslcert.ShortName(c), s.DaysLeft, extra)
		}
	}

	// verification
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Verification:")
	renderVerification(out, report)

	// OCSP
	if len(ocsp) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "OCSP:")
		renderOCSP(out, b, ocsp)
	}
	fmt.Fprintln(out)
}

func renderLeafBox(out io.Writer, s sslcert.Summary, full bool) {
	// 准备 rows
	rows := [][2]string{
		{"Subject", s.Subject},
		{"Issuer", s.Issuer},
		{"Valid", fmt.Sprintf("%s → %s  (%dd total)",
			s.NotBefore.UTC().Format("2006-01-02"), s.NotAfter.UTC().Format("2006-01-02"), s.ValidDays)},
		{"Days left", daysProgressBar(s.DaysLeft, s.ValidDays)},
		{"SAN", strings.Join(s.SAN, ", ")},
		{"Key", s.KeyAlgorithm},
		{"Signed", s.SigAlgorithm},
		{"Serial", truncateMiddle(s.Serial, 60)},
		{"SHA256", truncateMiddle(s.SHA256, 60)},
	}
	if full {
		if s.SHA1 != "" {
			rows = append(rows, [2]string{"SHA1", truncateMiddle(s.SHA1, 60)})
		}
		if len(s.IPAddresses) > 0 {
			rows = append(rows, [2]string{"IP SAN", strings.Join(s.IPAddresses, ", ")})
		}
		if len(s.KeyUsage) > 0 {
			rows = append(rows, [2]string{"Key Usage", strings.Join(s.KeyUsage, ", ")})
		}
		if len(s.ExtKeyUsage) > 0 {
			rows = append(rows, [2]string{"Ext Key Use", strings.Join(s.ExtKeyUsage, ", ")})
		}
		if len(s.OCSPServer) > 0 {
			rows = append(rows, [2]string{"OCSP URL", strings.Join(s.OCSPServer, ", ")})
		}
		if len(s.IssuingCertURL) > 0 {
			rows = append(rows, [2]string{"Issuer URL", strings.Join(s.IssuingCertURL, ", ")})
		}
		if len(s.CRLDistribution) > 0 {
			rows = append(rows, [2]string{"CRL URL", strings.Join(s.CRLDistribution, ", ")})
		}
	}

	// 找最长 label，对齐
	maxLabel := 0
	for _, r := range rows {
		if len(r[0]) > maxLabel {
			maxLabel = len(r[0])
		}
	}
	// 计算 box 宽度
	const totalWidth = 70
	contentWidth := totalWidth - 4 // 左 │  + 右  │

	fmt.Fprintln(out, "╭─ leaf "+strings.Repeat("─", totalWidth-8)+"╮")
	for _, r := range rows {
		label := r[0] + ":"
		label = padRight(label, maxLabel+2)
		valueWidth := contentWidth - len(label) - 1
		value := r[1]
		// 截断超长 value（保留视觉对齐）
		if visibleLen(value) > valueWidth {
			value = truncateMiddle(value, valueWidth)
		}
		line := fmt.Sprintf("│ %s %s", label, value)
		line = padRight(line, totalWidth-1) + "│"
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "╰"+strings.Repeat("─", totalWidth-2)+"╯")
}

// daysProgressBar 把剩余天数转成 ████████░░ 形式 + 数字。
func daysProgressBar(daysLeft, validDays int) string {
	if validDays <= 0 {
		return fmt.Sprintf("%d days", daysLeft)
	}
	if daysLeft < 0 {
		return fmt.Sprintf("EXPIRED  %d days ago", -daysLeft)
	}
	const barWidth = 10
	filled := (daysLeft * barWidth) / validDays
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	return fmt.Sprintf("%s  %d days", bar, daysLeft)
}

func renderVerification(out io.Writer, r *sslcert.VerificationReport) {
	mark := func(ok bool) string {
		if ok {
			return "✓"
		}
		return "✗"
	}
	// trusted
	if r.Trusted {
		fmt.Fprintln(out, "  ✓ chain trusted (system trust store)")
	} else {
		fmt.Fprintf(out, "  ✗ chain NOT trusted: %s\n", truncateOneLine(r.TrustErr, 80))
	}
	// hostname
	switch {
	case r.HostnameSkipped:
		fmt.Fprintln(out, "  ◇ hostname check skipped (file or empty SNI)")
	case r.HostnameOK:
		fmt.Fprintln(out, "  ✓ hostname matches SAN")
	default:
		fmt.Fprintf(out, "  ✗ hostname mismatch: %s\n", truncateOneLine(r.HostnameErr, 80))
	}
	// expiry
	switch {
	case r.Expired:
		fmt.Fprintln(out, "  ✗ EXPIRED")
	case r.NotYetValid:
		fmt.Fprintln(out, "  ✗ not yet valid (NotBefore in future)")
	default:
		fmt.Fprintln(out, "  ✓ not expired")
	}
	_ = mark
}

func renderOCSP(out io.Writer, b *sslcert.Bundle, statuses []sslcert.OCSPStatus) {
	chain := b.Chain
	for i, st := range statuses {
		if i >= len(chain) {
			break
		}
		c := chain[i]
		name := sslcert.ShortName(c)
		switch {
		case !st.Available:
			// 多见于 root；不打 noise
			if i+1 < len(chain) {
				fmt.Fprintf(out, "  ◇ %-22s no OCSP responder URL\n", name)
			}
		case st.Err != "":
			fmt.Fprintf(out, "  ⚠ %-22s OCSP failed: %s\n", name, truncateOneLine(st.Err, 60))
		case st.Revoked:
			fmt.Fprintf(out, "  ✗ %-22s REVOKED at %s (reason: %s)\n",
				name, st.RevokedAt.Format("2006-01-02"), st.Reason)
		case st.Status == "good":
			fmt.Fprintf(out, "  ✓ %-22s OCSP good\n", name)
		default:
			fmt.Fprintf(out, "  ◇ %-22s OCSP %s\n", name, st.Status)
		}
	}
}

// ─── 文本布局小工具 ─────────────────────────────────────────────────────

func padRight(s string, width int) string {
	if visibleLen(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen(s))
}

func visibleLen(s string) int {
	// 简单按 rune 数算，对 CJK 不完美但对我们这场景够（Subject/Issuer 都是 ASCII 占主）
	return len([]rune(s))
}

func truncateMiddle(s string, width int) string {
	rs := []rune(s)
	if len(rs) <= width {
		return s
	}
	if width < 5 {
		return string(rs[:width])
	}
	head := (width - 3) / 2
	tail := width - 3 - head
	return string(rs[:head]) + "..." + string(rs[len(rs)-tail:])
}

func truncateOneLine(s string, width int) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return truncateMiddle(s, width)
}
