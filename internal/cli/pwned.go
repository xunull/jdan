package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xunull/jdan/internal/pwned"
)

const (
	pwnedBaseURL = "https://api.pwnedpasswords.com/range/"
	pwnedUA      = "jdan-pwned (+https://github.com/xunull/jdan)"
	pwnedMaxBody = 4 << 20 // 4 MiB：一个 prefix 的返回最多上千条，远不到这
)

type pwnedCmdDeps struct {
	out     io.Writer
	errOut  io.Writer
	in      io.Reader
	exit    func(int)
	client  *http.Client
	baseURL string // 注入便于测试
}

func newPwnedCommand(deps pwnedCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	if deps.exit == nil {
		deps.exit = os.Exit
	}
	cmd := &cobra.Command{
		Use:   "pwned",
		Short: "查密码是否已泄露（HIBP k-匿名，密码不出本机）",
		Long: `查一个密码是否出现在已知数据泄露中，基于 Have I Been Pwned 的 Pwned Passwords API。

原理（k-匿名）：本地算 SHA1，只把哈希【前 5 位】发给服务器，服务器返回一批同前缀的
哈希后缀，本地再比对。你的明文密码和完整哈希都不离开本机。

例：
  jdan pwned                       # 无回显提示输入（密码不显示、不进 history）
  echo -n 'pw' | jdan pwned        # 从 stdin 读
  cat passwords.txt | jdan pwned --batch   # 逐行批量查（审计密码表）
  echo -n 'pw' | jdan pwned --json

退出码：泄露=1，干净=0，出错=2（可进 CI / pre-commit 当 gate）。
有意不提供 -p flag：查泄露的工具不该把你的密码留进 shell history。`,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			asJSON, _ := cmd.Flags().GetBool("json")
			batch, _ := cmd.Flags().GetBool("batch")
			noPadding, _ := cmd.Flags().GetBool("no-padding")

			client := deps.client
			if client == nil {
				client = &http.Client{Timeout: 15 * time.Second}
			}
			baseURL := deps.baseURL
			if baseURL == "" {
				baseURL = pwnedBaseURL
			}
			cfg := pwnedRun{
				client:  client,
				baseURL: baseURL,
				padding: !noPadding,
				cache:   map[string]string{},
			}

			if batch {
				return runPwnedBatch(deps, cfg, asJSON)
			}
			return runPwnedSingle(deps, cfg, asJSON)
		},
	}
	cmd.Flags().Bool("json", false, "结构化输出")
	cmd.Flags().Bool("batch", false, "从 stdin 逐行批量查")
	cmd.Flags().Bool("no-padding", false, "关闭 Add-Padding（默认开：返回定长，连响应大小都不泄露）")
	return cmd
}

type pwnedRun struct {
	client  *http.Client
	baseURL string
	padding bool
	cache   map[string]string // prefix → range body，批量时去重
}

func runPwnedSingle(deps pwnedCmdDeps, cfg pwnedRun, asJSON bool) error {
	pw, err := readPwnedPassword(deps.out, deps.in)
	if err != nil {
		return pwnedFail(deps, fmt.Errorf("读取密码失败：%w", err))
	}
	if pw == "" {
		return pwnedFail(deps, fmt.Errorf("未读到密码"))
	}
	count, err := cfg.check(pw)
	if err != nil {
		return pwnedFail(deps, err)
	}

	if asJSON {
		if err := writeIndentJSON(deps.out, map[string]any{
			"pwned": count > 0,
			"count": count,
		}); err != nil {
			return err
		}
	} else if count > 0 {
		fmt.Fprintf(deps.out, "⚠ 这个密码在已知泄露中出现过 %s 次 —— 强烈建议别再用\n", commafy(count))
	} else {
		fmt.Fprintln(deps.out, "✓ 没在 HIBP 数据集里出现过（注意：不等于绝对安全，只是没被收录）")
	}

	if count > 0 {
		deps.exit(1)
	}
	return nil
}

func runPwnedBatch(deps pwnedCmdDeps, cfg pwnedRun, asJSON bool) error {
	sc := bufio.NewScanner(deps.in)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	type row struct {
		Password string `json:"password"`
		Pwned    bool   `json:"pwned"`
		Count    int    `json:"count"`
	}
	var rows []row
	hit := 0
	for sc.Scan() {
		pw := strings.TrimRight(sc.Text(), "\r")
		if pw == "" {
			continue
		}
		count, err := cfg.check(pw)
		if err != nil {
			return pwnedFail(deps, err)
		}
		if count > 0 {
			hit++
		}
		rows = append(rows, row{Password: pw, Pwned: count > 0, Count: count})
	}
	if err := sc.Err(); err != nil {
		return pwnedFail(deps, err)
	}
	if len(rows) == 0 {
		return pwnedFail(deps, fmt.Errorf("stdin 没有可查的密码"))
	}

	if asJSON {
		if err := writeIndentJSON(deps.out, map[string]any{
			"total":  len(rows),
			"pwned":  hit,
			"checks": rows,
		}); err != nil {
			return err
		}
	} else {
		for _, r := range rows {
			if r.Pwned {
				fmt.Fprintf(deps.out, "⚠ %-24s 泄露 %s 次\n", r.Password, commafy(r.Count))
			} else {
				fmt.Fprintf(deps.out, "✓ %-24s 干净\n", r.Password)
			}
		}
		fmt.Fprintf(deps.out, "\n%d 个里有 %d 个已泄露\n", len(rows), hit)
	}

	if hit > 0 {
		deps.exit(1)
	}
	return nil
}

// check 查一个密码的泄露次数（带 prefix 缓存）。
func (c pwnedRun) check(pw string) (int, error) {
	prefix, suffix := pwned.SplitRange(pwned.SHA1Hex(pw))
	body, ok := c.cache[prefix]
	if !ok {
		var err error
		body, err = c.queryRange(prefix)
		if err != nil {
			return 0, err
		}
		if c.cache != nil {
			c.cache[prefix] = body
		}
	}
	return pwned.Lookup(body, suffix), nil
}

func (c pwnedRun) queryRange(prefix string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+prefix, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", pwnedUA) // HIBP 没 UA 会 403
	if c.padding {
		req.Header.Set("Add-Padding", "true")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 HIBP 失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HIBP 返回 %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, pwnedMaxBody))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// readPwnedPassword：真 TTY → 无回显提示；否则（管道/测试）读一行。
func readPwnedPassword(out io.Writer, in io.Reader) (string, error) {
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(out, "Password: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(out)
		return string(b), err
	}
	br := bufio.NewReader(in)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// pwnedFail 把错误打到 stderr 并以 exit code 2 结束（区别于泄露的 1）。
func pwnedFail(deps pwnedCmdDeps, err error) error {
	fmt.Fprintf(deps.errOut, "Error: %v\n", err)
	deps.exit(2)
	return nil
}

// commafy 给整数加千位逗号：10000000 → 10,000,000。
func commafy(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

func init() {
	rootCmd.AddCommand(newPwnedCommand(pwnedCmdDeps{}))
}
