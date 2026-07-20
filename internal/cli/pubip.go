package cli

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const (
	maxRetries = 3
)

type provider struct {
	name  string
	v4URL string
	v6URL string
}

var (
	ipifyProvider = provider{name: "ipify", v4URL: "https://api.ipify.org", v6URL: "https://api6.ipify.org"}
	ipipProvider  = provider{name: "ipip", v4URL: "https://myip.ipip.net", v6URL: "https://myip.ipip.net"}

	providerFlag string
)

var pubip4Cmd = &cobra.Command{
	Use:   "pubip4",
	Short: "查询本机公网 IPv4 地址",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := resolveProvider(cmd, "IPv4")
		return fetchPubIP(cmd, p.v4URL, "IPv4")
	},
}

var pubip6Cmd = &cobra.Command{
	Use:   "pubip6",
	Short: "查询本机公网 IPv6 地址",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := resolveProvider(cmd, "IPv6")
		return fetchPubIP(cmd, p.v6URL, "IPv6")
	},
}

func resolveProvider(cmd *cobra.Command, ipType string) provider {
	prov, _ := cmd.Flags().GetString("provider")
	switch prov {
	case "ipip":
		return ipipProvider
	default:
		return ipifyProvider
	}
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

		ip := parseIP(strings.TrimSpace(string(body)), cmd)
		fmt.Print(ip)
		return nil
	}

	return fmt.Errorf("无法获取 %s 地址: %w", ipType, lastErr)
}

func parseIP(body string, cmd *cobra.Command) string {
	prov, _ := cmd.Flags().GetString("provider")
	if prov == "ipip" {
		// body 格式: "当前 IP：1.2.3.4  来自于：..."
		parts := strings.SplitN(body, "来自于", 2)
		ip := strings.TrimSpace(parts[0])
		ip = strings.TrimPrefix(ip, "当前 IP：")
		return ip
	}
	// ipify 直接返回纯 IP 地址
	return body
}

func init() {
	pubip4Cmd.Flags().StringVarP(&providerFlag, "provider", "p", "ipify", `IP 查询服务：ipify（默认）或 ipip`)
	pubip6Cmd.Flags().StringVarP(&providerFlag, "provider", "p", "ipify", `IP 查询服务：ipify（默认）或 ipip`)
	rootCmd.AddCommand(pubip4Cmd)
	rootCmd.AddCommand(pubip6Cmd)
}
