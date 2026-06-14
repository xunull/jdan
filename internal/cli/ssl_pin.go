package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/sslcert"
)

type sslPinCmdDeps struct {
	out io.Writer
}

func newSSLPinCommand(deps sslPinCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "pin <host[:port]>",
		Short: "生成 cert pinning 用的 SPKI hash + 多种格式",
		Long: `算 cert 的 SPKI (Subject Public Key Info) hash，配合主流 cert pinning
格式：OkHttp (Android) / iOS NSAppTransportSecurity / HPKP HTTP header /
Mozilla NSS / curl --pinnedpubkey / 原始 base64。

⚠ 重要：cert pinning 用 SPKI hash，不是 cert fingerprint。
  cert renew (同 key) 后 SPKI hash 不变，pinning 不破；cert fingerprint 变了，pinning 就坏。
  所以 jdan ssl cert 显示的 SHA256 不能直接用来 pin。

默认 pin leaf + 第一个 intermediate（Apple/Android/Chrome 推荐 best practice）。

例：
  jdan ssl pin github.com                        # 默认所有 6 种格式
  jdan ssl pin example.com:8443 --format okhttp  # 只 OkHttp，给管道用
  jdan ssl pin example.com --leaf-only           # 只 leaf SPKI
  jdan ssl pin example.com --full                # chain 里所有 cert
  jdan ssl pin -f cert.pem                       # 本地 PEM 文件
  jdan ssl pin example.com --json                # 结构化输出`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSLPin(cmd, args, deps.out)
		},
	}
	cmd.Flags().StringP("file", "f", "", "从本地 PEM 文件读，不联网")
	cmd.Flags().String("sni", "", "TLS SNI 名（默认用 host）")
	cmd.Flags().String("format", "", "只输出指定格式：okhttp/ios/hpkp/nss/curl/raw")
	cmd.Flags().Bool("leaf-only", false, "只算 leaf SPKI hash")
	cmd.Flags().Bool("full", false, "chain 里所有 cert 都算（默认 leaf + 第一个 intermediate）")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().Duration("timeout", 5*time.Second, "TLS 握手超时")
	return cmd
}

func runSSLPin(cmd *cobra.Command, args []string, out io.Writer) error {
	filePath, _ := cmd.Flags().GetString("file")
	sni, _ := cmd.Flags().GetString("sni")
	format, _ := cmd.Flags().GetString("format")
	leafOnly, _ := cmd.Flags().GetBool("leaf-only")
	full, _ := cmd.Flags().GetBool("full")
	asJSON, _ := cmd.Flags().GetBool("json")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	if leafOnly && full {
		return errors.New("--leaf-only and --full are mutually exclusive")
	}

	// 拉 cert chain
	var (
		bundle *sslcert.Bundle
		host   string
		err    error
	)
	if filePath != "" {
		bundle, err = sslcert.ParsePEMFile(filePath)
	} else {
		if len(args) == 0 {
			return errors.New("missing host argument (or use -f file.pem)")
		}
		var port int
		host, port, err = sslcert.ParseTarget(args[0])
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		bundle, err = sslcert.FetchFromHost(ctx, sslcert.FetchOptions{
			Host: host, Port: port, SNI: sni, Timeout: timeout,
		})
	}
	if err != nil {
		return err
	}

	entries := selectEntries(bundle, leafOnly, full)
	if len(entries) == 0 {
		return errors.New("no certificates to pin")
	}

	// --format 单一格式输出（管道场景）
	if format != "" {
		f, ok := sslcert.PinFormatters[format]
		if !ok {
			return fmt.Errorf("unknown format %q (valid: %s)",
				format, strings.Join(sslcert.PinFormatNames, ", "))
		}
		fmt.Fprintln(out, f(host, entries))
		return nil
	}

	// --json
	if asJSON {
		payload := map[string]any{
			"source":  bundle.Source,
			"host":    host,
			"entries": entries,
			"formats": renderAllFormats(host, entries),
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	renderPinText(out, host, entries)
	return nil
}

// selectEntries 根据 flag 选 chain 里哪些 cert 出 SPKI：
//   - --leaf-only: 只 chain[0]
//   - --full:      所有 chain[i]
//   - 默认:        chain[0] + chain[1]（leaf + 第一个 intermediate）
func selectEntries(b *sslcert.Bundle, leafOnly, full bool) []sslcert.PinEntry {
	if b == nil || len(b.Chain) == 0 {
		return nil
	}
	chain := b.Chain

	var out []sslcert.PinEntry
	switch {
	case leafOnly:
		out = append(out, withRole(sslcert.EntryFromCert(chain[0]), "leaf"))
	case full:
		for i, c := range chain {
			out = append(out, withRole(sslcert.EntryFromCert(c), roleForIndex(i, len(chain), c.Subject.String() == c.Issuer.String())))
		}
	default:
		out = append(out, withRole(sslcert.EntryFromCert(chain[0]), "leaf"))
		if len(chain) > 1 {
			out = append(out, withRole(sslcert.EntryFromCert(chain[1]), "intermediate"))
		}
	}
	return out
}

func withRole(e sslcert.PinEntry, role string) sslcert.PinEntry {
	e.Role = role
	return e
}

func roleForIndex(i, n int, selfSigned bool) string {
	switch {
	case i == 0:
		return "leaf"
	case i == n-1 && selfSigned:
		return "root"
	}
	return "intermediate"
}

func renderAllFormats(host string, entries []sslcert.PinEntry) map[string]string {
	out := map[string]string{}
	for _, name := range sslcert.PinFormatNames {
		out[name] = sslcert.PinFormatters[name](host, entries)
	}
	return out
}

func renderPinText(out io.Writer, host string, entries []sslcert.PinEntry) {
	// 每个 cert 的 box
	for _, e := range entries {
		role := strings.ToUpper(e.Role[:1]) + e.Role[1:]
		fmt.Fprintf(out, "╭─ %s %s\n", role, strings.Repeat("─", 60-3-len(role)))
		fmt.Fprintf(out, "│ Subject:    CN=%s\n", e.SubjectCN)
		if e.IssuerCN != "" {
			fmt.Fprintf(out, "│ Issuer:     CN=%s\n", e.IssuerCN)
		}
		fmt.Fprintf(out, "│ SPKI hash:  %s\n", e.SPKISha256)
		fmt.Fprintln(out, "╰"+strings.Repeat("─", 60-2))
		fmt.Fprintln(out)
	}

	// 6 个格式
	fmt.Fprintln(out, "─── Pin formats ─────────────────────────────────────────────")
	fmt.Fprintln(out)
	for _, name := range sslcert.PinFormatNames {
		fmt.Fprintf(out, "▸ %s:\n", name)
		body := sslcert.PinFormatters[name](host, entries)
		for _, line := range strings.Split(body, "\n") {
			fmt.Fprintf(out, "    %s\n", line)
		}
		fmt.Fprintln(out)
	}
}

func init() {
	sslCmd.AddCommand(newSSLPinCommand(sslPinCmdDeps{}))
}
