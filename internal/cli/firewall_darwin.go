//go:build darwin

package cli

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// macOSFirewallHint 返回一个用户友好的提示字符串：如果当前 macOS 应用层防火墙开着，
// jdan 二进制（未 codesigned by Apple 团队）大概率会被默认 deny LAN 入站连接，
// 用户的 "localhost 通但 LAN IP 拒绝" 体验就是这个原因。
//
// 返回 "" 表示不需要提示（firewall off 或检测失败 / 不是 darwin）。
//
// 检测方式：调用 /usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate，
// 解析 "State = 1" / "State = 0" 部分。强制 LANG=C 避免本地化输出影响 grep。
//
// 实现注意：firewallCheckCmd 可以被测试替换，方便 unit test 不依赖系统状态。
var firewallCheckCmd = defaultFirewallCheckCmd

func defaultFirewallCheckCmd(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate")
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	return cmd.Output()
}

func macOSFirewallHint() string {
	// 防止 socketfilterfw 卡死整个 startup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := firewallCheckCmd(ctx)
	if err != nil {
		// 二进制不存在 / 权限不够 / 用户的 macOS 改造过——总之静默放过
		return ""
	}
	if !strings.Contains(string(out), "State = 1") {
		// State = 0 (disabled) 或 unknown，不打提示
		return ""
	}
	return "ℹ  macOS firewall is on; unsigned binaries may be blocked from LAN access.\n" +
		"   if LAN clients get \"connection refused\", see README §macOS firewall."
}
