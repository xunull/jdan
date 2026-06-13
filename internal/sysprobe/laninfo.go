// Package sysprobe 暴露"探查本机系统状态"的通用工具：网络接口、LAN IP
// 探测、操作系统防火墙状态。被多个子命令复用（jdan http serve 启动 banner、
// jdan net selfcheck、未来可能的诊断命令）。
//
// 设计目标：不联网，纯本机探查。所有需要发起网络请求的逻辑应放在调用方。
//
// laninfo.go 负责：枚举本机所有网络接口，挑出能让局域网内的其他设备访问到
// 本机服务器的 IP（RFC 1918 私有地址段）。
package sysprobe

import (
	"net"
	"sort"
)

// DetectLANIPs 返回本机所有可被局域网访问的 IPv4 地址。规则：
//   - 跳过未启用 (interface.Flags & FlagUp == 0)
//   - 跳过 loopback、point-to-point
//   - 跳过 IPv6 link-local
//   - 只保留 RFC 1918 私有 IPv4：10.0.0.0/8、172.16.0.0/12、192.168.0.0/16
//
// 排序：192.168.* 在前（家用最常见），其次 10.*，最后 172.16-31.*。
// 同一段内按字典序，保证多次调用结果稳定。
//
// 错误处理：如果 net.Interfaces() 本身报错，返回空 slice + error。
// 单个接口枚举失败不传播，跳过即可（这是常见情况：VPN 接口断开等）。
func DetectLANIPs() ([]net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagPointToPoint != 0 {
			// 跳过 PPP / VPN tap 之类
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := extractIPv4(addr)
			if ip == nil {
				continue
			}
			if !isRFC1918(ip) {
				continue
			}
			ips = append(ips, ip)
		}
	}
	sortLANIPs(ips)
	return ips, nil
}

func extractIPv4(addr net.Addr) net.IP {
	var ip net.IP
	switch v := addr.(type) {
	case *net.IPNet:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	default:
		return nil
	}
	ip = ip.To4()
	if ip == nil {
		return nil
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return nil
	}
	return ip
}

// isRFC1918 判断 IPv4 是否在私有地址段。
func isRFC1918(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	switch {
	case v4[0] == 10:
		return true
	case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
		return true
	case v4[0] == 192 && v4[1] == 168:
		return true
	}
	return false
}

// sortLANIPs 排序：192.168 → 10 → 172.16-31，组内字典序。
// 用 priority 分组，让"家用 WiFi 主网段"出现在最前。
func sortLANIPs(ips []net.IP) {
	priority := func(ip net.IP) int {
		v4 := ip.To4()
		if v4 == nil {
			return 9
		}
		switch {
		case v4[0] == 192 && v4[1] == 168:
			return 0
		case v4[0] == 10:
			return 1
		case v4[0] == 172:
			return 2
		}
		return 9
	}
	sort.SliceStable(ips, func(i, j int) bool {
		pi, pj := priority(ips[i]), priority(ips[j])
		if pi != pj {
			return pi < pj
		}
		return ips[i].String() < ips[j].String()
	})
}
