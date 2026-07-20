package cli

import (
	"context"
	"crypto/x509"
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

type sslCertCmdDeps struct {
	out io.Writer
}

// sslCertExitCode 让 runSSLCert 把"该 exit 1 的 expires-in 条件"
// 通过 cobra 的 RunE error 表达；main 的 zerolog 会打错误信息，但
// 我们 SilenceErrors 让它不重复打 banner。
type sslCertExitErr struct{ msg string }

func (e *sslCertExitErr) Error() string { return e.msg }

func newSSLCertCommand(deps sslCertCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "cert [host[:port]]",
		Short: "看 host 的 TLS 证书详情（chain + verification + OCSP）",
		Long: `从 host 取 TLS 证书或从本地 PEM 文件读，详细展示 leaf + chain，
做 trusted/hostname/expiry 三项验证，并查 OCSP revocation 状态。

例：
  jdan ssl cert github.com
  jdan ssl cert example.com:8443
  jdan ssl cert example.com --sni www.example.com   # 虚拟主机指定 SNI
  jdan ssl cert example.com --full                  # 展开 extensions
  jdan ssl cert example.com --json                  # 结构化输出
  jdan ssl cert example.com --pem                   # 输出标准 PEM 给管道
  jdan ssl cert -f cert.pem                         # 本地文件，不联网
  jdan ssl cert example.com --no-ocsp               # 跳过 OCSP（节省 ~300ms）
  jdan ssl cert example.com --expires-in 30d        # exit 1 if expires within（监控用）`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSSLCert(cmd, args, deps.out)
		},
	}
	cmd.Flags().StringP("file", "f", "", "从本地 PEM 文件读，不联网")
	cmd.Flags().String("sni", "", "TLS SNI 名（默认用 host）")
	cmd.Flags().Bool("full", false, "展开 extensions / key usage / 完整 chain")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	cmd.Flags().Bool("pem", false, "输出标准 PEM 给管道")
	cmd.Flags().Bool("no-ocsp", false, "跳过 OCSP 查询")
	cmd.Flags().Duration("timeout", 5*time.Second, "整体超时")
	cmd.Flags().String("expires-in", "", "如果 leaf 在此期内过期则 exit 1（如 30d / 720h）")
	return cmd
}

func runSSLCert(cmd *cobra.Command, args []string, out io.Writer) error {
	filePath, _ := cmd.Flags().GetString("file")
	sni, _ := cmd.Flags().GetString("sni")
	full, _ := cmd.Flags().GetBool("full")
	asJSON, _ := cmd.Flags().GetBool("json")
	asPEM, _ := cmd.Flags().GetBool("pem")
	noOCSP, _ := cmd.Flags().GetBool("no-ocsp")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	expiresIn, _ := cmd.Flags().GetString("expires-in")

	// source decision: --file 优先，否则需要 host arg
	var (
		bundle *sslcert.Bundle
		err    error
	)
	hostForVerify := ""
	if filePath != "" {
		bundle, err = sslcert.ParsePEMFile(filePath)
	} else {
		if len(args) == 0 {
			return errors.New("missing host argument (or use -f file.pem)")
		}
		host, port, perr := sslcert.ParseTarget(args[0])
		if perr != nil {
			return perr
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		bundle, err = sslcert.FetchFromHost(ctx, sslcert.FetchOptions{
			Host: host, Port: port, SNI: sni, Timeout: timeout,
		})
		hostForVerify = host
	}
	if err != nil {
		return err
	}

	// verify
	report := sslcert.Verify(bundle, hostForVerify)

	// OCSP（默认开；本地文件无意义因为没 issuer chain 时跳过）
	var ocspStatuses []sslcert.OCSPStatus
	if !noOCSP && filePath == "" {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ocspStatuses = sslcert.CheckChainOCSP(ctx, bundle.Chain)
	}

	// --pem 路径：直接 dump PEM，不渲染
	if asPEM {
		fmt.Fprint(out, sslcert.EncodePEM(bundle.FullChain()))
		return checkExpiresIn(bundle, expiresIn)
	}

	if asJSON {
		payload := map[string]any{
			"source":       bundle.Source,
			"leaf":         sslcert.Describe(bundle.Leaf()),
			"chain":        chainDescribed(bundle.FullChain()),
			"verification": report,
			"ocsp":         ocspStatuses,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return err
		}
		return checkExpiresIn(bundle, expiresIn)
	}

	renderCertText(out, bundle, report, ocspStatuses, full)
	return checkExpiresIn(bundle, expiresIn)
}

func chainDescribed(chain []*x509.Certificate) []sslcert.Summary {
	out := make([]sslcert.Summary, 0, len(chain))
	for _, c := range chain {
		out = append(out, sslcert.Describe(c))
	}
	return out
}

// checkExpiresIn 解析 duration（支持 "30d" / "720h" 这种），如果 leaf 在此期内过期 → 返回 sslCertExitErr 让 cobra 退出非 0。
func checkExpiresIn(b *sslcert.Bundle, spec string) error {
	if spec == "" {
		return nil
	}
	d, err := parseDurationWithDays(spec)
	if err != nil {
		return fmt.Errorf("invalid --expires-in: %w", err)
	}
	leaf := b.Leaf()
	if leaf == nil {
		return errors.New("no leaf cert to check expiry on")
	}
	threshold := sslcert.Now().Add(d)
	if leaf.NotAfter.Before(threshold) {
		return &sslCertExitErr{
			msg: fmt.Sprintf("leaf expires at %s (within %s)",
				leaf.NotAfter.Format("2006-01-02"), spec),
		}
	}
	return nil
}

// parseDurationWithDays 接受 "30d" / "720h" / "24h" 形式；time.ParseDuration
// 不支持 "d"，所以单独处理 days 后缀。
func parseDurationWithDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		num := strings.TrimSuffix(s, "d")
		days, err := time.ParseDuration(num + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	return time.ParseDuration(s)
}

func init() {
	sslCmd.AddCommand(newSSLCertCommand(sslCertCmdDeps{}))
}
