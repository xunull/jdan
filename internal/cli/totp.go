package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/totp"
)

// totpSecretEnv 是 secret 的环境变量来源（比 arg 安全，不进 shell history）。
const totpSecretEnv = "JDAN_TOTP_SECRET"

type totpCmdDeps struct {
	out io.Writer
	in  io.Reader
	now func() time.Time
	// getenv 注入便于测试 env 来源
	getenv func(string) string
}

func newTOTPCommand(deps totpCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.getenv == nil {
		deps.getenv = os.Getenv
	}
	cmd := &cobra.Command{
		Use:   "totp",
		Short: "TOTP 2FA 验证码（RFC 6238，兼容 Google Authenticator）",
		Long: `TOTP 工具集。生成 / 解析 / 验证 2FA 验证码。0 新依赖。

子命令：
  code <secret>       生成当前 6 位码
  uri <otpauth://>    解析 otpauth URI 并生成码（扫码得到的格式）
  verify <secret> <code>  验证一个码是否有效

安全：secret 是长期凭证。直接传 arg 会进 shell history + 进程列表(ps)，
只适合临时/测试。长期用请走 stdin（不带 secret 参数）或环境变量：
  echo "$SECRET" | jdan totp code -
  JDAN_TOTP_SECRET="$SECRET" jdan totp code

默认参数对齐 Google Authenticator：SHA1 / 6 位 / 30 秒。

例：
  jdan totp code JBSWY3DPEHPK3PXP
  jdan totp code JBSWY3DPEHPK3PXP --json
  jdan totp uri "otpauth://totp/GitHub:me?secret=JBSWY3DP&issuer=GitHub"
  jdan totp verify JBSWY3DPEHPK3PXP 283461`,
	}
	cmd.AddCommand(newTOTPCodeCommand(deps))
	cmd.AddCommand(newTOTPURICommand(deps))
	cmd.AddCommand(newTOTPVerifyCommand(deps))
	return cmd
}

// resolveSecret 按优先级取 secret：arg > env JDAN_TOTP_SECRET > stdin。
// stdin 来源时（无 arg、无 env）读一行。
func resolveSecret(args []string, deps totpCmdDeps) (string, error) {
	if len(args) > 0 && args[0] != "-" {
		return args[0], nil
	}
	if env := deps.getenv(totpSecretEnv); env != "" {
		return env, nil
	}
	// stdin
	r := bufio.NewReader(deps.in)
	line, err := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		if err != nil && err != io.EOF {
			return "", err
		}
		return "", errors.New("no secret provided (pass as arg, env JDAN_TOTP_SECRET, or stdin)")
	}
	return line, nil
}

func totpConfigFromFlags(cmd *cobra.Command) (totp.Config, error) {
	algoStr, _ := cmd.Flags().GetString("algo")
	digits, _ := cmd.Flags().GetInt("digits")
	period, _ := cmd.Flags().GetInt("period")
	algo, err := totp.ParseAlgorithm(algoStr)
	if err != nil {
		return totp.Config{}, err
	}
	return totp.Config{Digits: digits, Period: period, Algorithm: algo}, nil
}

func addTOTPParamFlags(cmd *cobra.Command) {
	cmd.Flags().String("algo", "sha1", "HMAC 算法：sha1/sha256/sha512")
	cmd.Flags().Int("digits", 6, "码位数（6 或 8）")
	cmd.Flags().Int("period", 30, "时间步长（秒）")
}

// ---- code ----

func newTOTPCodeCommand(deps totpCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "code [secret]",
		Short:         "生成当前 TOTP 码（secret: arg / env / stdin）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			cfg, err := totpConfigFromFlags(cmd)
			if err != nil {
				return err
			}
			secret, err := resolveSecret(args, deps)
			if err != nil {
				return err
			}
			return emitTOTPCode(deps, secret, cfg, asJSON)
		},
	}
	addTOTPParamFlags(cmd)
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func emitTOTPCode(deps totpCmdDeps, secret string, cfg totp.Config, asJSON bool) error {
	key, err := totp.DecodeSecret(secret)
	if err != nil {
		return err
	}
	unix := deps.now().Unix()
	code := totp.GenerateAt(key, unix, cfg)
	expires := totp.ExpiresInAt(unix, cfg)
	full := cfg // normalized copy for display
	if full.Period == 0 {
		full.Period = 30
	}
	if full.Digits == 0 {
		full.Digits = 6
	}
	if asJSON {
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"code":       code,
			"expires_in": expires,
			"period":     full.Period,
			"digits":     full.Digits,
		})
	}
	fmt.Fprintf(deps.out, "%s   (expires in %ds)\n", code, expires)
	return nil
}

// ---- uri ----

func newTOTPURICommand(deps totpCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "uri [otpauth-uri]",
		Short:         "解析 otpauth:// URI 并生成码",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			uri, err := resolveSecret(args, deps) // 复用：arg / stdin
			if err != nil {
				return err
			}
			return emitTOTPFromURI(deps, uri, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func emitTOTPFromURI(deps totpCmdDeps, uri string, asJSON bool) error {
	p, err := totp.ParseOtpauthURI(uri)
	if err != nil {
		return err
	}
	key, err := totp.DecodeSecret(p.Secret)
	if err != nil {
		return err
	}
	cfg := p.Config()
	unix := deps.now().Unix()
	code := totp.GenerateAt(key, unix, cfg)
	expires := totp.ExpiresInAt(unix, cfg)

	if asJSON {
		enc := json.NewEncoder(deps.out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{
			"issuer":     p.Issuer,
			"account":    p.Account,
			"algorithm":  string(cfg.Algorithm),
			"digits":     cfg.Digits,
			"period":     cfg.Period,
			"code":       code,
			"expires_in": expires,
		})
	}
	row := func(label, val string) {
		if val != "" {
			fmt.Fprintf(deps.out, "%-11s%s\n", label+":", val)
		}
	}
	row("Issuer", p.Issuer)
	row("Account", p.Account)
	row("Algorithm", string(cfg.Algorithm))
	fmt.Fprintf(deps.out, "%-11s%d\n", "Digits:", cfg.Digits)
	fmt.Fprintf(deps.out, "%-11s%ds\n", "Period:", cfg.Period)
	fmt.Fprintf(deps.out, "%-11s%s   (expires in %ds)\n", "Code:", code, expires)
	return nil
}

// ---- verify ----

type totpVerifyExitErr struct{ msg string }

func (e *totpVerifyExitErr) Error() string { return e.msg }

func newTOTPVerifyCommand(deps totpCmdDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "verify <secret> <code>",
		Short:         "验证一个码是否有效（退出码 0/1）",
		Args:          cobra.RangeArgs(1, 2),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			window, _ := cmd.Flags().GetInt("window")
			cfg, err := totpConfigFromFlags(cmd)
			if err != nil {
				return err
			}
			// 两种调用：verify <secret> <code> 或 verify <code>（secret 走 env/stdin）
			var secret, code string
			if len(args) == 2 {
				secret, code = args[0], args[1]
			} else {
				code = args[0]
				secret, err = resolveSecret(nil, deps)
				if err != nil {
					return err
				}
			}
			return emitTOTPVerify(deps, secret, code, window, cfg)
		},
	}
	addTOTPParamFlags(cmd)
	cmd.Flags().Int("window", 1, "容许前后各 N 个时间窗（时钟漂移容错）")
	return cmd
}

func emitTOTPVerify(deps totpCmdDeps, secret, code string, window int, cfg totp.Config) error {
	key, err := totp.DecodeSecret(secret)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)
	ok := totp.VerifyAt(key, code, deps.now().Unix(), window, cfg)
	if ok {
		fmt.Fprintln(deps.out, "✓ valid")
		return nil
	}
	fmt.Fprintln(deps.out, "✗ invalid")
	return &totpVerifyExitErr{msg: "code invalid"}
}

func init() {
	rootCmd.AddCommand(newTOTPCommand(totpCmdDeps{}))
}
