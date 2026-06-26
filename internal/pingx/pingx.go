// Package pingx 实现 jdan ping 子命令的核心：可指定 DNS server 解析域名再 ping。
//
// 设计：实际的 ICMP 由系统 ping 完成（shell out，像 jdan git 调 git），jdan 只负责
//   1) 用指定 DNS 把域名解析成 IP（复用 internal/dnslookup，含 DoH）
//   2) 构造 ping 的 argv
//   3) 尽力解析 ping 汇总行供 --json
// 0 新依赖（miekg/dns、cobra 都是项目已有）。仅 macOS + Linux。
package pingx

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/miekg/dns"

	"github.com/xunull/jdan/internal/dnslookup"
)

// Options 控制 ping argv 构造。
type Options struct {
	Count int      // -c N（<=0 不加，让 ping 用默认）
	V6    bool     // ping IPv6
	Extra []string // -- 之后透传给系统 ping 的原始参数
}

// BuildCommand 构造系统 ping 的可执行名 + 参数。target 是 IP 或 host。
// macOS 上 IPv6 用独立的 ping6 二进制；Linux 用 ping -6。
func BuildCommand(target string, opt Options, goos string) (string, []string) {
	bin := "ping"
	var args []string
	if opt.V6 {
		if goos == "darwin" {
			bin = "ping6" // macOS 的 ping6 不吃 -6
		} else {
			args = append(args, "-6")
		}
	}
	if opt.Count > 0 {
		args = append(args, "-c", strconv.Itoa(opt.Count))
	}
	args = append(args, opt.Extra...)
	args = append(args, target)
	return bin, args
}

// Resolve 用指定 DNS（resolver + server）把 host 解析成 IP。v6 决定查 A 还是 AAAA。
func Resolve(ctx context.Context, r dnslookup.Resolver, host, server string, v6 bool) (string, error) {
	qtype := dns.TypeA
	if v6 {
		qtype = dns.TypeAAAA
	}
	resp, err := r.Query(ctx, host, qtype, server)
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("DNS %s 无响应", server)
	}
	for _, rr := range resp.Answer {
		switch v := rr.(type) {
		case *dns.A:
			if !v6 {
				return v.A.String(), nil
			}
		case *dns.AAAA:
			if v6 {
				return v.AAAA.String(), nil
			}
		}
	}
	return "", fmt.Errorf("%s 在 DNS %s 上没有 %s 记录", host, server, dns.TypeToString[qtype])
}

// Summary 是 ping 汇总行的解析结果。
type Summary struct {
	Transmitted int
	Received    int
	LossPct     float64
	RTTMin      float64
	RTTAvg      float64
	RTTMax      float64
	HasStats    bool // packets 行解析成功
	HasRTT      bool // rtt 行解析成功
}

var (
	// "3 packets transmitted, 3 received, 0% packet loss"（Linux）
	// "3 packets transmitted, 3 packets received, 0.0% packet loss"（macOS）
	packetsRe = regexp.MustCompile(`(\d+) packets transmitted, (\d+)(?: packets)? received, ([\d.]+)% packet loss`)
	// "rtt min/avg/max/mdev = 11.8/12.0/12.1/0.1 ms"（Linux）
	// "round-trip min/avg/max/stddev = 11.8/12.0/12.1/0.1 ms"（macOS）
	rttRe = regexp.MustCompile(`(?:rtt|round-trip) min/avg/max\S* = ([\d.]+)/([\d.]+)/([\d.]+)`)
)

// ParseSummary 尽力解析 ping 输出的汇总行（Linux iputils + macOS BSD 两种格式）。
// 解析不出就让对应字段保持零值、HasStats/HasRTT 为 false。
func ParseSummary(out string) Summary {
	var s Summary
	if m := packetsRe.FindStringSubmatch(out); m != nil {
		s.Transmitted, _ = strconv.Atoi(m[1])
		s.Received, _ = strconv.Atoi(m[2])
		s.LossPct, _ = strconv.ParseFloat(m[3], 64)
		s.HasStats = true
	}
	if m := rttRe.FindStringSubmatch(out); m != nil {
		s.RTTMin, _ = strconv.ParseFloat(m[1], 64)
		s.RTTAvg, _ = strconv.ParseFloat(m[2], 64)
		s.RTTMax, _ = strconv.ParseFloat(m[3], 64)
		s.HasRTT = true
	}
	return s
}

// Runner 跑系统 ping，stdout/stderr 写到给定 writer，返回退出码。便于测试注入。
type Runner func(ctx context.Context, bin string, args []string, stdout, stderr io.Writer) (int, error)

// ExecRunner 是生产用 Runner：真实 exec 系统 ping。
func ExecRunner(ctx context.Context, bin string, args []string, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		// ping 非 0 退出（丢包 / 不可达）属正常返回，把退出码带回去
		return ee.ExitCode(), nil
	}
	if _, ok := err.(*exec.Error); ok {
		return -1, fmt.Errorf("找不到 %s 可执行文件，请确认系统已装 ping", bin)
	}
	return -1, err
}
