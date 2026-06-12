package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xunull/jdan/internal/jwtdecode"
)

type jwtCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newJWTCommand(deps jwtCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "jwt",
		Short: "JWT 工具集合",
		Long: `JWT 工具集合。

目前提供：
  jdan jwt decode <token>   纯本地解码 JWT header + payload（不验签、不联网）

将来可能加：
  jdan jwt verify           带 key 验签
  jdan jwt encode           构造 JWT`,
	}
	cmd.AddCommand(newJWTDecodeCommand(deps))
	return cmd
}

func newJWTDecodeCommand(deps jwtCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decode [token]",
		Short: "解码 JWT 的 header 和 payload",
		Long: `解码 JWT token。**不验证签名、不发任何网络请求**。

例：
  jdan jwt decode eyJhbGc...
  echo "$TOKEN" | jdan jwt decode
  jdan jwt decode "$TOKEN" --header-only
  jdan jwt decode "$TOKEN" --json     # 输出可被脚本消费的结构化结果
  jdan jwt decode "$TOKEN" --raw      # 不 pretty-print，原始 JSON

signature 段在文本输出里只显示 "(present, X bytes)"，避免误粘到日志里。
--json 输出会包含完整 signature base64url 字符串。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			headerOnly, _ := cmd.Flags().GetBool("header-only")
			asJSON, _ := cmd.Flags().GetBool("json")
			raw, _ := cmd.Flags().GetBool("raw")

			token, err := readJWTToken(deps.in, args)
			if err != nil {
				return err
			}
			r, err := jwtdecode.Decode(token)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(deps.out)
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}
			return renderJWTText(deps.out, r, headerOnly, raw)
		},
	}
	cmd.Flags().Bool("header-only", false, "只输出 header")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出（含 signature）")
	cmd.Flags().Bool("raw", false, "不 pretty-print，输出原始 JSON")
	return cmd
}

func readJWTToken(r io.Reader, args []string) (string, error) {
	if len(args) > 0 {
		return strings.TrimSpace(args[0]), nil
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", errors.New("no token: pass as argument or via stdin")
	}
	return s, nil
}

func renderJWTText(out io.Writer, r *jwtdecode.Result, headerOnly, raw bool) error {
	header := r.Header
	payload := r.Payload
	if raw {
		// raw 模式：把 map 重新 marshal（不带缩进）
		hb, _ := json.Marshal(r.HeaderMap)
		header = string(hb)
		pb, _ := json.Marshal(r.PayloadMap)
		payload = string(pb)
	}

	fmt.Fprintln(out, "Header:")
	fmt.Fprintln(out, indent(header, "  "))
	if headerOnly {
		return nil
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Payload:")
	fmt.Fprintln(out, indent(payload, "  "))
	fmt.Fprintln(out)

	// 摘要区
	if r.Algorithm != "" {
		fmt.Fprintf(out, "算法: %s\n", r.Algorithm)
	}
	if r.KeyID != "" {
		fmt.Fprintf(out, "Key ID: %s\n", r.KeyID)
	}
	if r.Subject != "" {
		fmt.Fprintf(out, "Subject: %s\n", r.Subject)
	}
	if r.Issuer != "" {
		fmt.Fprintf(out, "Issuer: %s\n", r.Issuer)
	}
	if len(r.Audience) > 0 {
		fmt.Fprintf(out, "Audience: %s\n", strings.Join(r.Audience, ", "))
	}
	if r.IssuedAt != nil {
		fmt.Fprintf(out, "签发: %s\n", r.IssuedAt.Format("2006-01-02 15:04:05 MST"))
	}
	if r.NotBefore != nil {
		fmt.Fprintf(out, "Not before: %s\n", r.NotBefore.Format("2006-01-02 15:04:05 MST"))
	}
	if r.ExpiresAt != nil {
		status := "剩余 " + r.TimeRemaining
		if r.Expired {
			status = "已过期"
		}
		fmt.Fprintf(out, "过期: %s (%s)\n", r.ExpiresAt.Format("2006-01-02 15:04:05 MST"), status)
	}
	fmt.Fprintf(out, "Signature: (present, %d chars base64url)\n", len(r.Signature))
	return nil
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func init() {
	rootCmd.AddCommand(newJWTCommand(jwtCmdDeps{}))
}
