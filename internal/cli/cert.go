package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/certgen"
)

type certCmdDeps struct {
	out io.Writer
}

func newCertCommand(deps certCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	cmd := &cobra.Command{
		Use:   "cert <name>",
		Short: "生成本地开发用的自签名 TLS 证书",
		Long: `生成本地开发 / 测试用的自签名 TLS 证书。0 新依赖（crypto/x509）。

替代记不住 flag 的 openssl req。默认就带正确的 SAN（现代浏览器要求 SAN，
光 CN 不行）。主参数自动进 SAN：是 IP 进 IP SAN，否则进 DNS SAN。

⚠ 仅限本地开发 / 测试，不要用于生产（生产证书走 ACME / certbot）。

例：
  jdan cert localhost                              # cert.pem + cert-key.pem
  jdan cert example.local --san "*.example.local"  # 额外 DNS SAN
  jdan cert myapp --ip 127.0.0.1,::1 --days 365
  jdan cert localhost --ca                         # 同时生成 CA（信任一次即可）
  jdan cert localhost --stdout                     # 输出到 stdout
  jdan cert localhost --key-type rsa`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCert(cmd, args[0], deps)
		},
	}
	cmd.Flags().String("san", "", "额外 DNS SAN（csv，支持 *.example.com 通配）")
	cmd.Flags().String("ip", "", "IP SAN（csv，如 127.0.0.1,::1）")
	cmd.Flags().Int("days", 825, "有效期天数（825 是浏览器接受的上限）")
	cmd.Flags().String("key-type", "ec", "私钥类型：ec / rsa / ed25519")
	cmd.Flags().String("out-dir", ".", "输出目录")
	cmd.Flags().String("prefix", "cert", "文件名前缀（→ <prefix>.pem / <prefix>-key.pem）")
	cmd.Flags().Bool("ca", false, "同时生成 CA 并用它签发（信任 CA 一次即可）")
	cmd.Flags().Bool("stdout", false, "输出到 stdout 而非文件")
	cmd.Flags().Bool("json", false, "输出元信息 JSON")
	return cmd
}

func runCert(cmd *cobra.Command, name string, deps certCmdDeps) error {
	sanStr, _ := cmd.Flags().GetString("san")
	ipStr, _ := cmd.Flags().GetString("ip")
	days, _ := cmd.Flags().GetInt("days")
	keyTypeStr, _ := cmd.Flags().GetString("key-type")
	outDir, _ := cmd.Flags().GetString("out-dir")
	prefix, _ := cmd.Flags().GetString("prefix")
	useCA, _ := cmd.Flags().GetBool("ca")
	toStdout, _ := cmd.Flags().GetBool("stdout")
	asJSON, _ := cmd.Flags().GetBool("json")

	keyType, err := certgen.ParseKeyType(keyTypeStr)
	if err != nil {
		return err
	}
	sans := certgen.BuildSANs(name, certgenSplit(sanStr), certgenSplit(ipStr))
	opts := certgen.Options{
		CommonName: name,
		SANs:       sans,
		Days:       days,
		KeyType:    keyType,
	}

	var (
		leaf *certgen.Result
		ca   *certgen.CA
	)
	if useCA {
		ca, err = certgen.GenerateCA(opts)
		if err != nil {
			return err
		}
		leaf, err = ca.SignLeaf(opts)
	} else {
		leaf, err = certgen.GenerateSelfSigned(opts)
	}
	if err != nil {
		return err
	}

	if toStdout {
		// cert 在前，key 在后；CA 时也附上
		deps.out.Write(leaf.CertPEM)
		deps.out.Write(leaf.KeyPEM)
		if ca != nil {
			deps.out.Write(ca.CertPEM)
			deps.out.Write(ca.KeyPEM)
		}
		return nil
	}

	// 写文件
	certPath := filepath.Join(outDir, prefix+".pem")
	keyPath := filepath.Join(outDir, prefix+"-key.pem")
	if err := writeCertFile(certPath, leaf.CertPEM, 0o644); err != nil {
		return err
	}
	if err := writeCertFile(keyPath, leaf.KeyPEM, 0o600); err != nil {
		return err
	}
	var caCertPath, caKeyPath string
	if ca != nil {
		caCertPath = filepath.Join(outDir, "ca.pem")
		caKeyPath = filepath.Join(outDir, "ca-key.pem")
		if err := writeCertFile(caCertPath, ca.CertPEM, 0o644); err != nil {
			return err
		}
		if err := writeCertFile(caKeyPath, ca.KeyPEM, 0o600); err != nil {
			return err
		}
	}

	if asJSON {
		payload := map[string]any{
			"cert":        certPath,
			"key":         keyPath,
			"subject":     leaf.Cert.Subject.CommonName,
			"san":         certgen.SANString(leaf.Cert),
			"key_type":    keyType.Label(),
			"not_before":  leaf.Cert.NotBefore.Format("2006-01-02T15:04:05Z07:00"),
			"not_after":   leaf.Cert.NotAfter.Format("2006-01-02T15:04:05Z07:00"),
			"fingerprint": certgen.FingerprintSHA256(leaf.Cert),
			"self_signed": ca == nil,
		}
		if ca != nil {
			payload["ca_cert"] = caCertPath
			payload["ca_key"] = caKeyPath
		}
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}

	renderCertResult(deps.out, leaf, ca, keyType, caCertPath, caKeyPath, days)
	return nil
}

func renderCertResult(w io.Writer, leaf *certgen.Result, ca *certgen.CA, keyType certgen.KeyType, caCertPath, caKeyPath string, days int) {
	if ca != nil {
		fmt.Fprintln(w, "Generated CA + leaf certificate:")
		fmt.Fprintf(w, "  CA cert:     %s        ← 加这个到信任库（一次）\n", caCertPath)
		fmt.Fprintf(w, "  CA key:      %s\n", caKeyPath)
		fmt.Fprintln(w, "  Leaf cert:   cert.pem")
		fmt.Fprintln(w, "  Leaf key:    cert-key.pem")
	} else {
		fmt.Fprintln(w, "Generated self-signed certificate:")
		fmt.Fprintln(w, "  Cert:        cert.pem")
		fmt.Fprintln(w, "  Key:         cert-key.pem")
	}
	c := leaf.Cert
	fmt.Fprintf(w, "  Subject:     CN=%s\n", c.Subject.CommonName)
	fmt.Fprintf(w, "  SAN:         %s\n", certgen.SANString(c))
	fmt.Fprintf(w, "  Key type:    %s\n", keyType.Label())
	fmt.Fprintf(w, "  Valid:       %s → %s (%d days)\n",
		c.NotBefore.Format("2006-01-02"), c.NotAfter.Format("2006-01-02"), days)
	fmt.Fprintf(w, "  Fingerprint: %s\n", certgen.FingerprintSHA256(c))
	fmt.Fprintln(w)
	if ca != nil {
		fmt.Fprintln(w, "⚠ Add ca.pem to your system/browser trust store once; then every cert")
		fmt.Fprintln(w, "  this CA signs is trusted. Keep ca-key.pem secret. Local dev only.")
	} else {
		fmt.Fprintln(w, "⚠ Self-signed: browsers will warn. Add cert.pem to your trust store,")
		fmt.Fprintln(w, "  or use --ca to generate a CA you can trust once. Local dev only.")
	}
}

func writeCertFile(path string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// certgenSplit 把 csv flag 值拆成 trim 后的非空段。
func certgenSplit(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func init() {
	rootCmd.AddCommand(newCertCommand(certCmdDeps{}))
}
