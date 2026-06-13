//go:build darwin

package sysprobe

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"
)

// FirewallStatus 描述 macOS 应用层防火墙的状态。
type FirewallStatus int

const (
	FirewallUnknown  FirewallStatus = iota // 不是 darwin 或检测失败
	FirewallDisabled                       // State = 0
	FirewallEnabled                        // State = 1 或 2
)

// FirewallCheckCmd 是可以被测试替换的全局变量。生产代码用 defaultFirewallCheckCmd
// 调 /usr/libexec/ApplicationFirewall/socketfilterfw --getglobalstate；测试可以
// 注入伪 output 验证解析逻辑。
var FirewallCheckCmd = defaultFirewallCheckCmd

func defaultFirewallCheckCmd(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate")
	cmd.Env = append(os.Environ(), "LANG=C", "LC_ALL=C")
	return cmd.Output()
}

// MacFirewallState 返回 firewall 当前状态。2 秒 timeout 防止 socketfilterfw
// 卡死调用方。
func MacFirewallState() FirewallStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := FirewallCheckCmd(ctx)
	if err != nil {
		return FirewallUnknown
	}
	s := string(out)
	if strings.Contains(s, "State = 1") || strings.Contains(s, "State = 2") {
		return FirewallEnabled
	}
	if strings.Contains(s, "State = 0") {
		return FirewallDisabled
	}
	return FirewallUnknown
}

// MacFirewallHint 返回给 jdan http serve 启动 banner 用的提示字符串。
// firewall off / unknown 时返回空串（不打扰用户）。
func MacFirewallHint() string {
	if MacFirewallState() != FirewallEnabled {
		return ""
	}
	return "ℹ  macOS firewall is on; unsigned binaries may be blocked from LAN access.\n" +
		"   if LAN clients get \"connection refused\", see README §macOS firewall."
}
