package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	jhash "github.com/xunull/jdan/internal/hash"
)

type hashCmdDeps struct {
	out io.Writer
	in  io.Reader
}

type hashCmdExitErr struct{ msg string }

func (e *hashCmdExitErr) Error() string { return e.msg }

func newHashCommand(deps hashCmdDeps) *cobra.Command {
	if deps.out == nil {
		deps.out = os.Stdout
	}
	if deps.in == nil {
		deps.in = os.Stdin
	}
	cmd := &cobra.Command{
		Use:   "hash <file>",
		Short: "计算文件的 md5/sha1/sha256/sha512（默认 sha256）",
		Long: `streaming hash（不全读进内存，1GB+ 文件也 OK）。
多算法时一遍读取并行算（io.MultiWriter 喂多个 hasher）。

例：
  jdan hash file.zip                       # sha256 (默认)
  jdan hash file.zip --algo md5            # 单算法
  jdan hash file.zip --algo md5,sha256     # 多算法一遍读取
  jdan hash file.zip --all                 # md5+sha1+sha256+sha512
  jdan hash file.zip --json                # 结构化输出
  echo "hi" | jdan hash -                  # stdin
  jdan hash file.zip --check checksums.txt # macOS shasum -c / Linux sha256sum -c 兼容`,
		Args:          cobra.MaximumNArgs(2), // file + 可选 checksum file
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHash(cmd, args, deps.out, deps.in)
		},
	}
	cmd.Flags().String("algo", "sha256", "算法 csv：md5/sha1/sha256/sha512")
	cmd.Flags().Bool("all", false, "跑全部 4 个算法（覆盖 --algo）")
	cmd.Flags().String("check", "", "校验模式：传 checksum 文件路径，对每行 hash+filename 验证")
	cmd.Flags().Bool("json", false, "结构化 JSON 输出")
	return cmd
}

func runHash(cmd *cobra.Command, args []string, out io.Writer, stdin io.Reader) error {
	checkPath, _ := cmd.Flags().GetString("check")
	algoStr, _ := cmd.Flags().GetString("algo")
	allAlgos, _ := cmd.Flags().GetBool("all")
	asJSON, _ := cmd.Flags().GetBool("json")

	// --check 模式
	if checkPath != "" {
		return runHashCheck(out, checkPath, asJSON)
	}

	// 普通 hash 模式
	if len(args) == 0 {
		return errors.New("missing file argument (or '-' for stdin)")
	}
	path := args[0]

	algos, err := selectAlgos(algoStr, allAlgos)
	if err != nil {
		return err
	}

	var reader io.Reader
	if path == "-" {
		reader = stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	}

	res, err := jhash.HashReader(reader, algos)
	if err != nil {
		return err
	}
	if path != "-" {
		res.Path = path
	}

	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	renderHashResult(out, res)
	return nil
}

func selectAlgos(algoStr string, allAlgos bool) ([]jhash.Algorithm, error) {
	if allAlgos {
		return jhash.AllAlgorithms(), nil
	}
	return jhash.ParseAlgorithms(algoStr)
}

func renderHashResult(out io.Writer, r *jhash.Result) {
	algos := r.SortedAlgos()
	if len(algos) == 1 {
		// 兼容 shasum / sha256sum 输出：`<hash>  <file>`
		path := r.Path
		if path == "" {
			path = "-"
		}
		fmt.Fprintf(out, "%s  %s\n", r.Sum(algos[0]), path)
		return
	}
	// 多算法：分行展示
	for _, a := range algos {
		fmt.Fprintf(out, "%-7s %s\n", strings.ToUpper(string(a))+":", r.Sum(a))
	}
	if r.Path != "" {
		fmt.Fprintf(out, "file:   %s\n", r.Path)
	}
}

// runHashCheck 实现 --check 模式：解析 checksum 文件，对每个 entry 跑 hash，对比。
// 输出格式跟 macOS shasum -c / Linux sha256sum -c 对齐，便于交叉验证。
func runHashCheck(out io.Writer, checkPath string, asJSON bool) error {
	f, err := os.Open(checkPath)
	if err != nil {
		return err
	}
	defer f.Close()

	entries, err := jhash.ParseChecksumFile(f)
	if err != nil {
		return err
	}

	results := make([]jhash.CheckResult, 0, len(entries))
	baseDir := filepath.Dir(checkPath) // 相对路径相对于 checksum 文件目录

	failed := 0
	for _, e := range entries {
		algo, err := jhash.AlgoFromHexLength(e.Expected)
		if err != nil {
			results = append(results, jhash.CheckResult{
				Entry: e, Status: "ERROR", Err: err.Error(),
			})
			failed++
			continue
		}

		path := e.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}

		fp, ferr := os.Open(path)
		if ferr != nil {
			results = append(results, jhash.CheckResult{
				Entry: e, Status: "MISSING", Err: ferr.Error(),
			})
			failed++
			continue
		}
		res, herr := jhash.HashReader(fp, []jhash.Algorithm{algo})
		_ = fp.Close()
		if herr != nil {
			results = append(results, jhash.CheckResult{
				Entry: e, Status: "ERROR", Err: herr.Error(),
			})
			failed++
			continue
		}
		got := res.Sum(algo)
		cr := jhash.CheckResult{Entry: e, Got: got}
		if strings.EqualFold(got, e.Expected) {
			cr.Status = "OK"
		} else {
			cr.Status = "FAILED"
			failed++
		}
		results = append(results, cr)
	}

	if asJSON {
		payload := map[string]any{
			"check_file": checkPath,
			"total":      len(results),
			"failed":     failed,
			"results":    results,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(payload)
	} else {
		for _, r := range results {
			fmt.Fprintf(out, "%s: %s\n", r.Entry.Path, r.Status)
			if r.Status != "OK" && r.Err != "" {
				fmt.Fprintf(out, "  (%s)\n", r.Err)
			}
		}
		fmt.Fprintf(out, "\n%d total, %d failed\n", len(results), failed)
	}

	if failed > 0 {
		return &hashCmdExitErr{
			msg: fmt.Sprintf("%d checksum(s) failed", failed),
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(newHashCommand(hashCmdDeps{}))
}
