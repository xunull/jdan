package netprobe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"
)

// ResolveDetail 是 resolve 阶段的特有字段。
type ResolveDetail struct {
	Hostname  string   `json:"hostname"`
	IPs       []net.IP `json:"ips"`
	IsLiteral bool     `json:"is_literal"`         // 输入本身就是 IP literal，无需 DNS
	Resolver  string   `json:"resolver,omitempty"` // 实际用的 resolver
}

func runResolve(ctx context.Context, t *Target, opts Options) *StageResult {
	start := time.Now()
	r := &StageResult{Stage: StageResolve}
	d := &ResolveDetail{Hostname: t.Host}

	if ip := net.ParseIP(t.Host); ip != nil {
		d.IPs = []net.IP{ip}
		d.IsLiteral = true
		r.Success = true
		r.Detail = t.Host + " (literal IP)"
		r.Duration = time.Since(start)
		r.Resolve = d
		return r
	}

	resolver, resolverName := buildResolver(opts.Resolver)
	d.Resolver = resolverName

	dnsCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	ips, err := resolver.LookupIP(dnsCtx, "ip", t.Host)
	r.Duration = time.Since(start)
	if err != nil {
		r.Success = false
		r.Class = ClassifyDNSError(err)
		r.Detail = "failed: " + err.Error()
		r.Err = err.Error()
		r.Explanation = WhatItMeans(r.Class)
		r.Hint = HintForClass(r.Class)
		r.Resolve = d
		return r
	}
	if len(ips) == 0 {
		r.Success = false
		r.Class = ClassDNSNoSuchHost
		r.Err = "no addresses returned"
		r.Explanation = WhatItMeans(r.Class)
		r.Hint = HintForClass(r.Class)
		r.Resolve = d
		return r
	}

	sortIPs(ips)
	d.IPs = ips
	r.Success = true
	resolverDisplay := d.Resolver
	if resolverDisplay == "" || resolverDisplay == "system" {
		resolverDisplay = "system resolver"
	}
	r.Detail = fmt.Sprintf("%s → %d record(s) via %s", t.Host, len(ips), resolverDisplay)
	r.Resolve = d
	return r
}

// buildResolver 根据 opts.Resolver 决定用系统 resolver（默认）还是指定 server。
// 指定 server 时构造一个走 Go 内部 resolver 的 *net.Resolver。
func buildResolver(spec string) (*net.Resolver, string) {
	if spec == "" {
		return net.DefaultResolver, "system"
	}
	// spec 可能是 "8.8.8.8" 或 "8.8.8.8:53"
	server := spec
	if _, _, err := net.SplitHostPort(spec); err != nil {
		server = net.JoinHostPort(spec, "53")
	}
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, "udp", server)
		},
	}
	return r, server
}

// sortIPs 把 IPv4 排前面（用户更常关心），IPv6 排后面，组内字典序。
func sortIPs(ips []net.IP) {
	sort.SliceStable(ips, func(i, j int) bool {
		i4 := ips[i].To4() != nil
		j4 := ips[j].To4() != nil
		if i4 != j4 {
			return i4
		}
		return ips[i].String() < ips[j].String()
	})
}

func summarizeIPs(ips []net.IP) string {
	if len(ips) == 1 {
		return ips[0].String()
	}
	v4, v6 := 0, 0
	for _, ip := range ips {
		if ip.To4() != nil {
			v4++
		} else {
			v6++
		}
	}
	if v4 > 0 && v6 > 0 {
		return joinNetIPs(ips, ", ")
	}
	return joinNetIPs(ips, ", ")
}

func joinNetIPs(ips []net.IP, sep string) string {
	if len(ips) == 0 {
		return ""
	}
	parts := make([]string, len(ips))
	for i, ip := range ips {
		parts[i] = ip.String()
	}
	return strings_join(parts, sep)
}

// 小工具避免 import strings 仅为一个 join；保持依赖最小
func strings_join(parts []string, sep string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	n := len(sep) * (len(parts) - 1)
	for _, p := range parts {
		n += len(p)
	}
	b := make([]byte, 0, n)
	b = append(b, parts[0]...)
	for _, p := range parts[1:] {
		b = append(b, sep...)
		b = append(b, p...)
	}
	return string(b)
}

// errResolveTimeout 是 hints.go 检查用的桩，让 timeout 类错误能被识别。
var errResolveTimeout = errors.New("resolver: i/o timeout")
