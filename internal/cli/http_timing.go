package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/xunull/jdan/internal/httptiming"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "HTTP 相关子命令",
}

var httpTimingCmd = &cobra.Command{
	Use:   "timing [url]",
	Short: "测量 HTTP 请求各阶段耗时",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		n, _ := cmd.Flags().GetInt("n")
		jsonOut, _ := cmd.Flags().GetBool("json")
		insecure, _ := cmd.Flags().GetBool("insecure")

		if n < 1 {
			return fmt.Errorf("-n 必须大于等于 1")
		}

		var transport http.RoundTripper
		if insecure {
			transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}

		var results []httptiming.Result
		var lastErr error
		for i := 0; i < n; i++ {
			r, err := httptiming.Measure(context.Background(), url, transport)
			if err != nil {
				log.Warn().Int("round", i+1).Err(err).Msg("请求失败")
				lastErr = err
				continue
			}
			results = append(results, r)
		}

		if len(results) == 0 {
			return fmt.Errorf("所有 %d 次请求均失败: %v", n, lastErr)
		}

		if jsonOut {
			out, err := httptiming.FormatJSON(results)
			if err != nil {
				return err
			}
			fmt.Println(out)
		} else {
			fmt.Print(httptiming.FormatText(results))
		}
		return nil
	},
}

func init() {
	httpTimingCmd.Flags().IntP("n", "n", 1, "请求次数（默认 1，大于 1 时输出平均值）")
	httpTimingCmd.Flags().Bool("json", false, "以 JSON 格式输出")
	httpTimingCmd.Flags().BoolP("insecure", "k", false, "跳过 TLS 证书验证")
	httpCmd.AddCommand(httpTimingCmd)
	rootCmd.AddCommand(httpCmd)
}
