package cli

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xunull/jdan/internal/htpasswdx"
)

type htpasswdCmdDeps struct {
	out io.Writer
	in  io.Reader
}

func newHtpasswdCommand(deps htpasswdCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "htpasswd <user>",
		Short: "生成 / 校验 Apache·nginx Basic Auth 密码哈希（bcrypt/apr1/SHA）",
		Long: `生成 Basic Auth 的 user:hash 行（Apache/nginx .htpasswd 用）。0 新依赖。

算法（默认 bcrypt）：
  默认       bcrypt $2y$    最安全（x/crypto）
  --apr1     $apr1$...      Apache MD5-crypt，老系统通吃
  --sha      {SHA}...       无盐，不安全，仅兼容

密码只走无回显输入：TTY 下交互输入（输两次确认），非 TTY 从 stdin 读一行。
绝不收 -p 明文参数（避免进 shell history）。

例：
  jdan htpasswd alice                       # 交互输密码 → 打印 alice:$2y$...
  jdan htpasswd alice --apr1                 # 用 apr1
  printf 'pass\n' | jdan htpasswd alice      # 非 TTY：stdin 读密码
  jdan htpasswd alice -f .htpasswd           # upsert 进文件（替换同名 / 追加新）
  jdan htpasswd --verify '$2y$10$...'        # 校验：输密码，比对已有 hash`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			useAPR1, _ := cmd.Flags().GetBool("apr1")
			useSHA, _ := cmd.Flags().GetBool("sha")
			cost, _ := cmd.Flags().GetInt("cost")
			file, _ := cmd.Flags().GetString("file")
			verify, _ := cmd.Flags().GetString("verify")

			if useAPR1 && useSHA {
				return fmt.Errorf("--apr1 和 --sha 只能选一个")
			}

			// 校验模式
			if verify != "" {
				pw, err := readSecret(deps, "密码: ", false)
				if err != nil {
					return err
				}
				ok, err := htpasswdx.Verify(verify, pw)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("✗ 密码不匹配")
				}
				fmt.Fprintln(deps.out, "✓ 密码匹配")
				return nil
			}

			// 生成模式：需要 user
			if len(args) != 1 {
				return fmt.Errorf("需要一个用户名参数（或用 --verify 进入校验模式）")
			}
			user := args[0]

			pw, err := readSecret(deps, "新密码: ", true)
			if err != nil {
				return err
			}

			var hash string
			switch {
			case useSHA:
				hash = htpasswdx.SHA1(pw)
			case useAPR1:
				salt, err := apr1Salt()
				if err != nil {
					return err
				}
				hash = htpasswdx.APR1(pw, salt)
			default:
				hash, err = htpasswdx.Bcrypt(pw, cost)
				if err != nil {
					return fmt.Errorf("bcrypt 失败（cost 需 4–31）：%w", err)
				}
			}

			line := user + ":" + hash
			if file != "" {
				return upsertFile(deps.out, file, user, line)
			}
			fmt.Fprintln(deps.out, line)
			return nil
		},
	}
	cmd.Flags().Bool("apr1", false, "用 Apache apr1（MD5-crypt）")
	cmd.Flags().Bool("sha", false, "用 {SHA}（无盐，不安全，仅兼容）")
	cmd.Flags().Int("cost", htpasswdx.DefaultCost, "bcrypt cost（4–31）")
	cmd.Flags().StringP("file", "f", "", "upsert 进 htpasswd 文件（替换同名用户 / 追加新用户）")
	cmd.Flags().String("verify", "", "校验模式：传一个已有 hash，再输密码比对")
	return cmd
}

// readSecret：TTY 下无回显读密码（confirm 时输两次比对）；非 TTY 从 deps.in 读一行。
func readSecret(deps htpasswdCmdDeps, prompt string, confirm bool) (string, error) {
	if f, ok := deps.in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		pw, err := promptNoEcho(f, prompt)
		if err != nil {
			return "", err
		}
		if confirm {
			again, err := promptNoEcho(f, "再输一次: ")
			if err != nil {
				return "", err
			}
			if again != pw {
				return "", fmt.Errorf("两次输入不一致")
			}
		}
		return pw, nil
	}
	// 非 TTY（管道 / 测试）：读一行
	return readPasswordLine(deps.in)
}

func promptNoEcho(f *os.File, prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(os.Stderr)
	return string(b), err
}

// apr1Salt 生成 8 字符 apr1 salt（crypt(3) 字母表）。
func apr1Salt() (string, error) {
	const set = "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = set[int(b[i])%len(set)]
	}
	return string(b), nil
}

func upsertFile(out io.Writer, path, user, line string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	updated := htpasswdx.Upsert(string(content), user, line)
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return err
	}
	fmt.Fprintf(out, "已写入 %s（用户 %s）\n", path, user)
	return nil
}

func init() {
	rootCmd.AddCommand(newHtpasswdCommand(htpasswdCmdDeps{}))
}
