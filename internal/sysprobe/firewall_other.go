//go:build !darwin

package sysprobe

// FirewallStatus 在非 darwin 平台上保留 enum 形状（编译一致），但只有
// FirewallUnknown 一个合法值。
type FirewallStatus int

const (
	FirewallUnknown  FirewallStatus = iota
	FirewallDisabled                // 仅为类型完整性保留，Linux/Windows 不返回这个值
	FirewallEnabled
)

// MacFirewallState 在非 darwin 平台上始终返回 FirewallUnknown。
// Linux 的 iptables / Windows Defender 行为完全不同，留给未来按平台单独实现。
func MacFirewallState() FirewallStatus { return FirewallUnknown }

// MacFirewallHint 在非 darwin 平台上是 no-op。
func MacFirewallHint() string { return "" }
