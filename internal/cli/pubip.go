package cli

import (
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const (
	pubip4URL  = "https://api.ipify.org"
	pubip6URL  = "https://api6.ipify.org"
	maxRetries = 3
)

var pubip4Cmd = &cobra.Command{
	Use:   "pubip4",
	Short: "查询本机公网 IPv4 地址",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fetchPubIP(cmd, pubip4URL, "IPv4")
	},
}

var pubip6Cmd = &cobra.Command{
	Use:   "pubip6",
	Short: "查询本机公网 IPv6 地址",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fetchPubIP(cmd, pubip6URL, "IPv6")
	},
}

func fetchPubIP(cmd *cobra.Command, url, ipType string) error {
	client := &http.Client{}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(url)
		if err != nil {
			log.Warn().Int("retry", i+1).Err(err).Msg("请求失败")
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Warn().Int("retry", i+1).Err(err).Msg("读取响应失败")
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Warn().Int("retry", i+1).Int("status", resp.StatusCode).Msg("请求返回非 200 状态")
			lastErr = fmt.Errorf("请求返回状态码 %d", resp.StatusCode)
			continue
		}

		fmt.Print(string(body))
		return nil
	}

	return fmt.Errorf("无法获取 %s 地址: %w", ipType, lastErr)
}

func init() {
	rootCmd.AddCommand(pubip4Cmd)
	rootCmd.AddCommand(pubip6Cmd)
}
