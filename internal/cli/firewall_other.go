//go:build !darwin

package cli

// macOSFirewallHint 在非 darwin 平台上是 no-op。Linux / Windows 没有同款的
// "macOS Application Firewall by-binary block" 行为；标准 iptables / Windows
// Defender 也会拦但语义和触发条件完全不同，不在这个 hint 的范围里。
func macOSFirewallHint() string { return "" }
