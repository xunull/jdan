package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/gitleaksx"
	"github.com/xunull/jdan/internal/gitx"
)

// errGitleaksNotInstalled 表示机器上没有 gitleaks 可执行文件。
var errGitleaksNotInstalled = errors.New("gitleaks 未安装")

// gitleaks 发现泄露时的约定退出码（我们用 --exit-code 1 固定）。
const gitleaksLeakExitCode = 1

const gitleaksInstallHint = `未检测到 gitleaks。jdan git secrets 的检测交给 gitleaks，请先安装：
    macOS:  brew install gitleaks
    其他:   https://github.com/gitleaks/gitleaks#installing`

// gitleaksFunc 跑一次 gitleaks 扫描，返回 JSON 报告、是否有泄露、错误。
// leaks=true 表示 gitleaks 以「发现泄露」退出（exit 1），不是运行错误。便于测试注入。
type gitleaksFunc func(dir string, args []string) (report string, leaks bool, err error)

type gitSecretsDeps struct {
	out      io.Writer
	errOut   io.Writer
	run      gitx.Runner  // git，用于文件名审计 + 仓库判断；nil → 真实 git
	gitleaks gitleaksFunc // nil → 真实 gitleaks
	exit     func(int)    // nil → os.Exit
	dir      string       // "" → 当前目录
}

func newGitSecretsCommand(deps gitSecretsDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.errOut == nil {
		deps.errOut = os.Stderr
	}
	if deps.run == nil {
		deps.run = gitx.ExecRunner
	}
	if deps.gitleaks == nil {
		deps.gitleaks = realGitleaks
	}
	if deps.exit == nil {
		deps.exit = os.Exit
	}
	if deps.dir == "" {
		deps.dir = "."
	}

	cmd := &cobra.Command{
		Use:   "secrets [path]",
		Short: "扫 git 历史里被提交过的密钥/凭据（底层用 gitleaks）",
		Long: `扫 git 仓库历史里是否提交过密钥/凭据（AWS/GitHub/Slack/私钥…），也能扫暂存区。

检测交给 gitleaks（需先安装），jdan 负责：默认脱敏、补一层「敏感文件名」审计、
统一输出与退出码。0 新 Go 依赖（运行时需要 git + gitleaks）。

例：
  jdan git secrets                              扫当前仓库全历史
  jdan git secrets /path/to/repo                扫指定仓库
  jdan git secrets --staged                     只扫暂存区（pre-commit 用）
  jdan git secrets --log-opts=origin/main..HEAD 限定范围
  jdan git secrets --json                       机读（同样脱敏）
  jdan git secrets --show-secrets               输出明文（默认脱敏）

当 pre-commit hook 用：.git/hooks/pre-commit 里写一行
    exec jdan git secrets --staged
之后每次 git commit 前自动扫暂存区，有泄露就拦下。

退出码：0 干净 / 1 有发现（CI 可卡门）/ 2 环境缺失（没装 gitleaks 或非 git 仓库）。
有意不做：不替你改写历史（只检测 + 提示轮换）、不联网验真、不重造 gitleaks 规则引擎。`,
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := deps.dir
			if len(args) == 1 {
				dir = args[0]
			}

			staged, _ := cmd.Flags().GetBool("staged")
			showSecrets, _ := cmd.Flags().GetBool("show-secrets")
			noFilenames, _ := cmd.Flags().GetBool("no-filenames")
			asJSON, _ := cmd.Flags().GetBool("json")
			logOpts, _ := cmd.Flags().GetString("log-opts")
			baseline, _ := cmd.Flags().GetString("baseline")

			// 先确认是 git 仓库，给出比 gitleaks 更友好的报错。
			if !gitx.IsRepo(deps.run, dir) {
				fmt.Fprintln(deps.errOut, "不是 git 仓库（试试 git init，或指定正确的 path）")
				deps.exit(2)
				return nil
			}

			mode := "history"
			if staged {
				mode = "staged"
			}

			glArgs := []string{
				"--report-format", "json", "--report-path", "-",
				"--no-banner", "--exit-code", "1", "--log-level", "error",
			}
			if showSecrets {
				glArgs = append(glArgs, "--redact=0")
			} else {
				glArgs = append(glArgs, "--redact=100")
			}
			if staged {
				glArgs = append(glArgs, "--staged")
			}
			if logOpts != "" {
				glArgs = append(glArgs, "--log-opts="+logOpts)
			}
			if baseline != "" {
				glArgs = append(glArgs, "--baseline-path", baseline)
			}

			report, _, err := deps.gitleaks(dir, glArgs)
			if err != nil {
				if errors.Is(err, errGitleaksNotInstalled) {
					fmt.Fprintln(deps.errOut, gitleaksInstallHint)
				} else {
					fmt.Fprintf(deps.errOut, "gitleaks 运行失败：%v\n", err)
				}
				deps.exit(2)
				return nil
			}

			content, err := gitleaksx.ParseReport([]byte(report))
			if err != nil {
				fmt.Fprintf(deps.errOut, "%v\n", err)
				deps.exit(2)
				return nil
			}

			res := gitleaksx.Result{Mode: mode, Content: content}
			if !noFilenames {
				if paths, err := addedPaths(deps.run, dir, staged); err == nil {
					res.Files = gitleaksx.AuditFilenames(paths)
				}
			}

			if asJSON {
				s, err := res.FormatJSON()
				if err != nil {
					fmt.Fprintf(deps.errOut, "%v\n", err)
					deps.exit(2)
					return nil
				}
				fmt.Fprintln(deps.out, s)
			} else {
				renderGitSecretsText(deps.out, deps.errOut, res)
			}

			if res.Detected() {
				deps.exit(1)
			}
			return nil
		},
	}
	cmd.Flags().Bool("staged", false, "只扫暂存区（pre-commit 用），而非全历史")
	cmd.Flags().Bool("show-secrets", false, "输出明文密钥（默认脱敏）")
	cmd.Flags().Bool("no-filenames", false, "跳过敏感文件名审计层")
	cmd.Flags().Bool("json", false, "结构化输出（默认同样脱敏）")
	cmd.Flags().String("log-opts", "", "透传给 gitleaks 的 git log 选项（限范围，如 origin/main..HEAD）")
	cmd.Flags().String("baseline", "", "gitleaks baseline 文件（忽略已知项）")
	return cmd
}

// realGitleaks 真实调用 gitleaks 可执行文件：gitleaks git <dir> <args...>。
// 报告走 --report-path -（stdout），日志走 stderr。
func realGitleaks(dir string, args []string) (string, bool, error) {
	path, err := exec.LookPath("gitleaks")
	if err != nil {
		return "", false, errGitleaksNotInstalled
	}
	full := append([]string{"git", dir}, args...)
	cmd := exec.Command(path, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil {
		return stdout.String(), false, nil // exit 0 = 干净
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == gitleaksLeakExitCode {
		return stdout.String(), true, nil // exit 1 = 有泄露（非运行错误）
	}
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		msg = err.Error()
	}
	return "", false, fmt.Errorf("%s", msg)
}

// addedPaths 返回「曾被新增过的路径」：全历史（--all）或仅暂存区（--staged 时）。
func addedPaths(run gitx.Runner, dir string, staged bool) ([]string, error) {
	var (
		out string
		err error
	)
	if staged {
		out, err = run(dir, "diff", "--cached", "--diff-filter=A", "--name-only")
	} else {
		out, err = run(dir, "log", "--all", "--diff-filter=A", "--pretty=format:", "--name-only")
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

func renderGitSecretsText(out, errOut io.Writer, r gitleaksx.Result) {
	if s := r.Render(); s != "" {
		fmt.Fprint(out, s)
	}
	nc, nf := len(r.Content), len(r.Files)
	if nc == 0 && nf == 0 {
		fmt.Fprintln(errOut, "未发现被提交过的密钥/凭据 ✓")
		return
	}
	fmt.Fprintf(errOut, "\n共 %d 处内容命中 + %d 个可疑文件（已脱敏；exit 1）\n", nc, nf)
	fmt.Fprintln(errOut, "提醒：如确为泄露，先轮换对应凭据；是否清理历史（git-filter-repo/BFG）由你决定，jdan 不代改。")
}
