package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/secretscan"
)

type secretsScanCmdDeps struct {
	out    io.Writer
	errOut io.Writer
	in     io.Reader
	exit   func(int)
}

// 走目录时默认跳过的目录、文件名（-a 不跳）。
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true,
	"build": true, ".next": true, "target": true, ".venv": true,
	"venv": true, "__pycache__": true, ".idea": true,
}

var skipFiles = map[string]bool{
	"package-lock.json": true, "yarn.lock": true, "pnpm-lock.yaml": true,
	"go.sum": true, "Cargo.lock": true, "composer.lock": true,
	"Gemfile.lock": true, "poetry.lock": true,
}

const maxScanFileSize = 5 << 20 // 5 MiB

type located struct {
	file string
	secretscan.Finding
}

func newSecretsScanCommand(deps secretsScanCmdDeps) *cobra.Command {
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
		Use:   "secrets-scan [path...]",
		Short: "扫文件里疑似硬编码的密钥/token（输出脱敏）",
		Long: `扫文本里疑似硬编码的密钥/token：正则引擎（已知格式，高精度）+ 高熵引擎
（未知 token）。0 新依赖。输出永不含完整 secret，只给脱敏预览（前 4…后 4）。

例：
  jdan secrets-scan .            递归扫当前目录
  jdan secrets-scan config/      扫某目录
  jdan secrets-scan a.env b.go   扫指定文件
  cat x | jdan secrets-scan      扫 stdin
  jdan secrets-scan . --json     机读输出（也不含完整 secret）

退出码：0 无发现 / 1 有发现（CI 可卡门）/ 2 出错。
降噪：默认跳过 .git node_modules vendor 等目录、二进制、lock 文件（-a 全扫）；
内嵌 allowlist（UUID/示例占位）；行内 ` + "`# pragma: allowlist secret`" + ` 豁免。
git 历史扫描有意未做（v1），后续 --history。`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			all, _ := cmd.Flags().GetBool("all")
			noEntropy, _ := cmd.Flags().GetBool("no-entropy")
			minEntropy, _ := cmd.Flags().GetFloat64("min-entropy")
			asJSON, _ := cmd.Flags().GetBool("json")

			opts := secretscan.Options{NoEntropy: noEntropy, MinEntropy: minEntropy}

			var findings []located
			scanData := func(file string, data []byte) {
				for _, f := range secretscan.ScanBytes(data, opts) {
					findings = append(findings, located{file: file, Finding: f})
				}
			}

			if len(args) == 0 {
				data, err := io.ReadAll(deps.in)
				if err != nil {
					fmt.Fprintf(deps.errOut, "读取 stdin 失败：%v\n", err)
					deps.exit(2)
					return nil
				}
				scanData("(stdin)", data)
			} else {
				if err := scanPaths(args, all, scanData); err != nil {
					fmt.Fprintf(deps.errOut, "%v\n", err)
					deps.exit(2)
					return nil
				}
			}

			sort.Slice(findings, func(i, j int) bool {
				if findings[i].file != findings[j].file {
					return findings[i].file < findings[j].file
				}
				return findings[i].Line < findings[j].Line
			})

			if asJSON {
				emitSecretsJSON(deps.out, findings)
			} else {
				emitSecretsText(deps.out, deps.errOut, findings)
			}

			if len(findings) > 0 {
				deps.exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolP("all", "a", false, "不跳过 .git/node_modules/二进制/lock 文件")
	cmd.Flags().Bool("no-entropy", false, "关闭高熵引擎（只用正则）")
	cmd.Flags().Float64("min-entropy", 4.0, "高熵阈值（bits/byte），越高越保守")
	cmd.Flags().Bool("json", false, "结构化输出（同样不含完整 secret）")
	return cmd
}

func scanPaths(paths []string, all bool, scan func(file string, data []byte)) error {
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("无法访问 %q：%w", p, err)
		}
		if !info.IsDir() {
			// 显式指定的文件：照扫（不套目录跳过规则），仅守大小/二进制
			if data, ok := readScannable(p, all); ok {
				scan(p, data)
			}
			continue
		}
		err = filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // 单个条目读不动就跳过，别中断整次扫描
			}
			if d.IsDir() {
				if !all && skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !all && skipFiles[d.Name()] {
				return nil
			}
			if data, ok := readScannable(path, all); ok {
				scan(path, data)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// readScannable 读文件，跳过超大/二进制（除非 all）。
func readScannable(path string, all bool) ([]byte, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if !all && info.Size() > maxScanFileSize {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if !all && isBinary(data) {
		return nil, false
	}
	return data, true
}

func isBinary(data []byte) bool {
	n := min(len(data), 8000)
	return bytes.IndexByte(data[:n], 0) >= 0
}

func emitSecretsText(out, errOut io.Writer, findings []located) {
	for _, f := range findings {
		loc := fmt.Sprintf("%s:%d", f.file, f.Line)
		if f.Entropy > 0 {
			fmt.Fprintf(out, "%s  [%s]  %s  (%s, entropy %.1f)\n", loc, f.Rule, f.Redacted, f.Confidence, f.Entropy)
		} else {
			fmt.Fprintf(out, "%s  [%s]  %s  (%s)\n", loc, f.Rule, f.Redacted, f.Confidence)
		}
	}
	if len(findings) == 0 {
		fmt.Fprintln(errOut, "未发现疑似密钥 ✓")
	} else {
		fmt.Fprintf(errOut, "\n共 %d 处疑似密钥（已脱敏；exit 1）\n", len(findings))
	}
}

func emitSecretsJSON(out io.Writer, findings []located) {
	arr := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		m := map[string]any{
			"file":       f.file,
			"line":       f.Line,
			"col":        f.Col,
			"rule":       f.Rule,
			"redacted":   f.Redacted,
			"confidence": f.Confidence,
		}
		if f.Entropy > 0 {
			m["entropy"] = f.Entropy
		}
		arr = append(arr, m)
	}
	_ = writeIndentJSON(out, arr)
}

func init() {
	rootCmd.AddCommand(newSecretsScanCommand(secretsScanCmdDeps{}))
}
