// Package dnstrace 实现 jdan dns trace 子命令的迭代 DNS 解析：
// 从根服务器开始，沿 referral 一路追到权威 NS，展示每一跳的委派路径。
//
// 设计要点（与 dig +trace 对齐 + 与现有 jdan 命令族协调）：
//   - 13 台 root server IP 硬编码（RFC 数据，20 年只动过 1 次）。
//   - Sequential NS selection（保持输出线性可读，便于人类扫视）。
//   - 主路径直接 UDP/TCP 到权威 NS，不走 recursive resolver——这是 trace 的本质。
//   - glueless NS 通过外部 bootstrap Resolver 解析：
//     CLI 层在传 --doh 时注入 dohResolver，否则注入 osLookupResolver（包装 net.LookupIP）。
//   - 输出形态对齐 jdan dns lookup / reverse：text + --json + --short + --verbose。
//
// trace.go 单文件组织：roots + 类型 + Tracer + format + bootstrap wrapper，
// 通过 Step 0 的 8 文件复杂度门槛。
package dnstrace

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/miekg/dns"

	"github.com/xunull/jdan/internal/dnslookup"
)

// ------ Root server 表 ------

// rootServer 是一台 RFC 公开的根 DNS 服务器。
type rootServer struct {
	Name string
	IPv4 string
}

// rootServers 是 13 台根服务器的 IPv4 列表，按字母序。
//
// 数据源：https://www.iana.org/domains/root/servers (RFC 7720)。
// 历史：1980s 至今只发生过 1 次 IP 变更（b.root-servers.net 在 2017，再次在 2023）。
// 若再有变化，本表 patch 一行即可。
//
// v1 仅 IPv4——足以覆盖 99% 的网络环境。IPv6-only 网络场景待未来需求触发。
var rootServers = []rootServer{
	{"a.root-servers.net.", "198.41.0.4"},
	{"b.root-servers.net.", "170.247.170.2"},
	{"c.root-servers.net.", "192.33.4.12"},
	{"d.root-servers.net.", "199.7.91.13"},
	{"e.root-servers.net.", "192.203.230.10"},
	{"f.root-servers.net.", "192.5.5.241"},
	{"g.root-servers.net.", "192.112.36.4"},
	{"h.root-servers.net.", "198.97.190.53"},
	{"i.root-servers.net.", "192.36.148.17"},
	{"j.root-servers.net.", "192.58.128.30"},
	{"k.root-servers.net.", "193.0.14.129"},
	{"l.root-servers.net.", "199.7.83.42"},
	{"m.root-servers.net.", "202.12.27.33"},
}

// ------ 公共类型 ------

// HopType 是一跳的分类：referral / answer / error。
type HopType string

const (
	HopReferral HopType = "REFERRAL" // 该 NS 把我们指向下一 zone
	HopAnswer   HopType = "ANSWER"   // 该 NS 给出了最终答案
	HopError    HopType = "ERROR"    // 该 NS 网络错误或返回不可用
)

// Hop 是 trace 中的一跳记录。
type Hop struct {
	Zone         string              `json:"zone"`                    // 当前 zone（"."、"com."、"example.com." 等）
	ServerName   string              `json:"server_name"`             // NS hostname（root 时为 "a.root-servers.net." 等）
	ServerIP     string              `json:"server_ip"`               // 实际 dial 的 IP
	QueryTimeMs  int64               `json:"query_time_ms"`           // 该跳查询耗时
	Type         HopType             `json:"type"`                    // REFERRAL / ANSWER / ERROR
	Rcode        string              `json:"rcode,omitempty"`         // dns rcode 名（NOERROR / NXDOMAIN ...）
	ReferralZone string              `json:"referral_zone,omitempty"` // referral 指向的下一 zone
	NSReferrals  []string            `json:"ns_referrals,omitempty"`  // referral 的 NS 名列表
	GlueIPs      map[string][]string `json:"glue_ips,omitempty"`      // in-bailiwick glue: NS name → IP 列表
	Answers      []string            `json:"answers,omitempty"`       // ANSWER 类型：渲染后的 RR 列表
	Error        string              `json:"error,omitempty"`         // ERROR 类型：短错误标识
}

// Result 是一次 Trace 调用的完整结果。
type Result struct {
	Domain      string `json:"domain"`
	QueryType   string `json:"query_type"`
	TotalTimeMs int64  `json:"total_time_ms"`
	Hops        []Hop  `json:"hops"`
	Final       *Hop   `json:"final,omitempty"` // 指向 Hops 中的 ANSWER 元素副本；中断时为 nil
}

// Options 控制 Tracer 行为。
type Options struct {
	// Bootstrap 用于 glueless NS hostname → IP 解析。
	// CLI 层根据 --doh 决定注入 dnslookup.NewDoHResolver(...) 还是 NewOSLookupResolver(...)。
	// 为 nil 时 trace 遇 glueless 直接中断。
	Bootstrap dnslookup.Resolver

	HopTimeout   time.Duration // 单跳查询超时；<=0 用默认 3s
	TotalTimeout time.Duration // 整 trace 超时；<=0 用默认 30s
	MaxHops      int           // 最大跳数；<=0 用默认 20
	StartServer  string        // 起步 server IP；空表示用 rootServers 列表
}

// ------ Transport 抽象（mockable）------

// transport 把"问 NS 一个问题"抽为接口，方便在测试中替换为 fake，无需打真实网络。
type transport interface {
	Exchange(ctx context.Context, msg *dns.Msg, server string) (resp *dns.Msg, rtt time.Duration, err error)
}

// miekgTransport 是生产用 transport，包装 miekg/dns 的 UDP+TCP 客户端，处理 TC 截断重试。
type miekgTransport struct {
	udp *dns.Client
	tcp *dns.Client
}

func newMiekgTransport(hopTimeout time.Duration) transport {
	return &miekgTransport{
		udp: &dns.Client{Net: "udp", Timeout: hopTimeout},
		tcp: &dns.Client{Net: "tcp", Timeout: hopTimeout},
	}
}

func (t *miekgTransport) Exchange(ctx context.Context, msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
	resp, rtt, err := t.udp.ExchangeContext(ctx, msg, server)
	if err != nil {
		return nil, rtt, err
	}
	if resp != nil && resp.Truncated {
		// UDP 响应被截断，按 RFC 1035 切到 TCP 重试。
		return t.tcp.ExchangeContext(ctx, msg, server)
	}
	return resp, rtt, nil
}

// ------ Tracer ------

// Tracer 是 trace 主对象。线程不安全（包含临时状态），每次 Trace 调用独立构造。
type Tracer struct {
	transport    transport
	bootstrap    dnslookup.Resolver
	hopTimeout   time.Duration
	totalTimeout time.Duration
	maxHops      int
	startServer  string
}

// NewTracer 构造一个走真实网络的 Tracer。
func NewTracer(opts Options) *Tracer {
	return newTracerWithTransport(opts, newMiekgTransport(defaultHopTimeout(opts.HopTimeout)))
}

// newTracerWithTransport 是测试入口，允许注入 fake transport。
func newTracerWithTransport(opts Options, tr transport) *Tracer {
	return &Tracer{
		transport:    tr,
		bootstrap:    opts.Bootstrap,
		hopTimeout:   defaultHopTimeout(opts.HopTimeout),
		totalTimeout: defaultTotalTimeout(opts.TotalTimeout),
		maxHops:      defaultMaxHops(opts.MaxHops),
		startServer:  opts.StartServer,
	}
}

func defaultHopTimeout(v time.Duration) time.Duration {
	if v <= 0 {
		return 3 * time.Second
	}
	return v
}

func defaultTotalTimeout(v time.Duration) time.Duration {
	if v <= 0 {
		return 30 * time.Second
	}
	return v
}

func defaultMaxHops(v int) int {
	if v <= 0 {
		return 20
	}
	return v
}

// nsTarget 是 trace 中候选要查询的"下一跳"：NS hostname + IP。
type nsTarget struct {
	Name string
	IP   string
}

// initialNSs 返回起步 NS 列表：要么用 --server 指定的单一 server，要么 13 根。
func (t *Tracer) initialNSs() []nsTarget {
	if t.startServer != "" {
		return []nsTarget{{Name: "(custom)", IP: t.startServer}}
	}
	out := make([]nsTarget, 0, len(rootServers))
	for _, rs := range rootServers {
		out = append(out, nsTarget{Name: rs.Name, IP: rs.IPv4})
	}
	return out
}

// Trace 执行迭代解析。返回 *Result（可能含中断信息）+ error（仅配置错误才返回 error；
// 网络/协议错误记录在 Result.Hops 的 ERROR hop 中）。
func (t *Tracer) Trace(ctx context.Context, domain string, qtype uint16) (*Result, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain 不能为空")
	}
	domain = dns.Fqdn(domain)
	qtypeStr := dns.TypeToString[qtype]
	if qtypeStr == "" {
		qtypeStr = fmt.Sprintf("TYPE%d", qtype)
	}

	result := &Result{
		Domain:    domain,
		QueryType: qtypeStr,
	}

	ctx, cancel := context.WithTimeout(ctx, t.totalTimeout)
	defer cancel()

	start := time.Now()
	defer func() { result.TotalTimeMs = time.Since(start).Milliseconds() }()

	visited := map[string]bool{".": true}
	currentZone := "."
	currentNSs := t.initialNSs()

	for hopIdx := 0; hopIdx < t.maxHops; hopIdx++ {
		hop := t.querySomeNS(ctx, currentZone, currentNSs, domain, qtype)
		result.Hops = append(result.Hops, hop)

		switch hop.Type {
		case HopError:
			return result, nil
		case HopAnswer:
			final := result.Hops[len(result.Hops)-1]
			result.Final = &final
			return result, nil
		case HopReferral:
			if visited[hop.ReferralZone] {
				result.Hops = append(result.Hops, Hop{
					Zone:  hop.ReferralZone,
					Type:  HopError,
					Error: "cycle detected: referral 指向已访问 zone",
				})
				return result, nil
			}
			visited[hop.ReferralZone] = true
			currentZone = hop.ReferralZone

			nextNSs, err := t.resolveReferralNS(ctx, hop)
			if err != nil {
				result.Hops = append(result.Hops, Hop{
					Zone:  currentZone,
					Type:  HopError,
					Error: err.Error(),
				})
				return result, nil
			}
			currentNSs = nextNSs
		}
	}

	// max hops 用尽仍无 ANSWER
	result.Hops = append(result.Hops, Hop{
		Type:  HopError,
		Error: fmt.Sprintf("max hops %d exceeded without final answer", t.maxHops),
	})
	return result, nil
}

// querySomeNS 在当前 zone 的 NS 列表里 sequential fallback。
// 任一 NS 给出 REFERRAL 或 ANSWER 即返回；全部 ERROR 才返回最后一个错误。
func (t *Tracer) querySomeNS(ctx context.Context, zone string, nss []nsTarget, domain string, qtype uint16) Hop {
	if len(nss) == 0 {
		return Hop{Zone: zone, Type: HopError, Error: "no NS available for zone"}
	}
	var last Hop
	for _, ns := range nss {
		hop := t.queryOneNS(ctx, zone, ns, domain, qtype)
		if hop.Type != HopError {
			return hop
		}
		last = hop
	}
	return last
}

// queryOneNS 向单个 NS 发出一次 non-recursive 查询，把响应分类为 REFERRAL/ANSWER/ERROR。
func (t *Tracer) queryOneNS(ctx context.Context, zone string, ns nsTarget, domain string, qtype uint16) Hop {
	hop := Hop{
		Zone:       zone,
		ServerName: ns.Name,
		ServerIP:   ns.IP,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(domain, qtype)
	msg.RecursionDesired = false // trace 不要 recursive：直接看 NS 自己怎么回

	server := joinHostPort(ns.IP, "53")

	hopCtx, cancel := context.WithTimeout(ctx, t.hopTimeout)
	defer cancel()

	resp, rtt, err := t.transport.Exchange(hopCtx, msg, server)
	hop.QueryTimeMs = rtt.Milliseconds()

	if err != nil {
		hop.Type = HopError
		hop.Error = friendlyTraceErr(err)
		return hop
	}
	if resp == nil {
		hop.Type = HopError
		hop.Error = "EMPTY_RESPONSE"
		return hop
	}

	hop.Rcode = dns.RcodeToString[resp.Rcode]
	if hop.Rcode == "" {
		hop.Rcode = fmt.Sprintf("RCODE%d", resp.Rcode)
	}

	// 优先看 Answer section
	if len(resp.Answer) > 0 {
		hop.Type = HopAnswer
		for _, rr := range resp.Answer {
			if v := renderRR(rr); v != "" {
				hop.Answers = append(hop.Answers, v)
			}
		}
		return hop
	}

	// 否则看 Authority 是否包含 NS（referral）
	if resp.Rcode == dns.RcodeSuccess && hasNSReferral(resp.Ns) {
		hop.Type = HopReferral
		var refZone string
		hop.GlueIPs = make(map[string][]string)
		for _, rr := range resp.Ns {
			if nsRR, ok := rr.(*dns.NS); ok {
				hop.NSReferrals = append(hop.NSReferrals, nsRR.Ns)
				if refZone == "" {
					refZone = nsRR.Hdr.Name
				}
			}
		}
		hop.ReferralZone = refZone
		// 从 Additional 提取 in-bailiwick glue
		for _, rr := range resp.Extra {
			switch v := rr.(type) {
			case *dns.A:
				hop.GlueIPs[v.Hdr.Name] = append(hop.GlueIPs[v.Hdr.Name], v.A.String())
			case *dns.AAAA:
				hop.GlueIPs[v.Hdr.Name] = append(hop.GlueIPs[v.Hdr.Name], v.AAAA.String())
			}
		}
		return hop
	}

	// rcode 非 NOERROR，或 NOERROR 但无 answer 也无 NS = 异常终止
	hop.Type = HopError
	hop.Error = fmt.Sprintf("%s: no answer or referral", hop.Rcode)
	return hop
}

// hasNSReferral 判断 Authority section 是否含 NS RR。
func hasNSReferral(rrs []dns.RR) bool {
	for _, rr := range rrs {
		if _, ok := rr.(*dns.NS); ok {
			return true
		}
	}
	return false
}

// resolveReferralNS 根据上一跳的 REFERRAL，构造下一跳的 NS 候选列表。
//
// 优先级：
//  1. in-bailiwick glue（referral 自带 IP）—— 零额外查询
//  2. glueless：调 bootstrap Resolver 解析 NS hostname 为 IP
//  3. 都失败 → 返回 error，trace 中断
func (t *Tracer) resolveReferralNS(ctx context.Context, refHop Hop) ([]nsTarget, error) {
	var out []nsTarget
	for _, nsName := range refHop.NSReferrals {
		if ips, ok := refHop.GlueIPs[nsName]; ok {
			for _, ip := range ips {
				out = append(out, nsTarget{Name: nsName, IP: ip})
			}
		}
	}
	if len(out) > 0 {
		return out, nil
	}

	// glueless 路径
	if t.bootstrap == nil {
		return nil, fmt.Errorf("glueless NS 但未配置 bootstrap resolver: %v", refHop.NSReferrals)
	}
	for _, nsName := range refHop.NSReferrals {
		msg, err := t.bootstrap.Query(ctx, nsName, dns.TypeA, "")
		if err != nil || msg == nil {
			continue
		}
		for _, rr := range msg.Answer {
			if a, ok := rr.(*dns.A); ok && a.A != nil {
				out = append(out, nsTarget{Name: nsName, IP: a.A.String()})
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("bootstrap 无法解析任何 NS hostname: %v", refHop.NSReferrals)
	}
	return out, nil
}

// ------ 错误短化 ------

// friendlyTraceErr 把网络层错误翻译为短标识，便于在 hop 表格里显示。
func friendlyTraceErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "context canceled"):
		return "TIMEOUT"
	case strings.Contains(msg, "connection refused"):
		return "CONNECTION_REFUSED"
	case strings.Contains(msg, "no route to host"):
		return "NO_ROUTE"
	case strings.Contains(msg, "network is unreachable"):
		return "NETWORK_UNREACHABLE"
	default:
		return msg
	}
}

// ------ Render RR (与 dnslookup 同款，私有副本) ------

// renderRR 把单条 RR 渲染为面向用户的紧凑字符串。
func renderRR(rr dns.RR) string {
	switch v := rr.(type) {
	case *dns.A:
		return v.A.String()
	case *dns.AAAA:
		return v.AAAA.String()
	case *dns.MX:
		return fmt.Sprintf("%d %s", v.Preference, v.Mx)
	case *dns.TXT:
		var b strings.Builder
		for i, s := range v.Txt {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(strconv.Quote(s))
		}
		return b.String()
	case *dns.CNAME:
		return v.Target
	case *dns.NS:
		return v.Ns
	case *dns.PTR:
		return v.Ptr
	case *dns.SOA:
		return fmt.Sprintf("%s %s %d %d %d %d %d",
			v.Ns, v.Mbox, v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)
	default:
		// 退路：用 miekg 默认 String() 去掉前缀 metadata
		s := rr.String()
		count := 0
		for i, c := range s {
			if c == '\t' {
				count++
				if count == 4 {
					return strings.TrimSpace(s[i+1:])
				}
			}
		}
		return s
	}
}

// joinHostPort 给 IPv4 / IPv6 IP 拼上端口（IPv6 加方括号）。
func joinHostPort(host, port string) string {
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]:" + port
	}
	return host + ":" + port
}

// ------ Bootstrap resolver wrapper（OS lookup → dnslookup.Resolver） ------

// osLookupResolver 把 net.DefaultResolver 包装为 dnslookup.Resolver 接口。
// 仅支持 TypeA / TypeAAAA——glueless NS bootstrap 只需要解析 hostname → IP。
type osLookupResolver struct {
	timeout time.Duration
}

// NewOSLookupResolver 返回一个用 OS 系统 resolver 做 hostname→IP 的 Resolver 实现。
// 用于 trace 在未传 --doh 时的 glueless NS bootstrap。可能被本地劫持，但 trace 多数
// 路径在 in-bailiwick glue 下不触发此 path。
func NewOSLookupResolver(timeout time.Duration) dnslookup.Resolver {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &osLookupResolver{timeout: timeout}
}

func (r *osLookupResolver) Query(ctx context.Context, domain string, qtype uint16, _ string) (*dns.Msg, error) {
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return nil, fmt.Errorf("os lookup 仅支持 A/AAAA，传入 %s", dns.TypeToString[qtype])
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	network := "ip4"
	if qtype == dns.TypeAAAA {
		network = "ip6"
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, network, strings.TrimSuffix(domain, "."))
	if err != nil {
		return nil, err
	}

	resp := new(dns.Msg)
	resp.Rcode = dns.RcodeSuccess
	for _, ip := range ips {
		switch qtype {
		case dns.TypeA:
			if v4 := ip.To4(); v4 != nil {
				resp.Answer = append(resp.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: dns.Fqdn(domain), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
					A:   v4,
				})
			}
		case dns.TypeAAAA:
			if ip.To4() == nil {
				resp.Answer = append(resp.Answer, &dns.AAAA{
					Hdr:  dns.RR_Header{Name: dns.Fqdn(domain), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
					AAAA: ip,
				})
			}
		}
	}
	return resp, nil
}

// ------ Format 函数 ------

// FormatText 渲染默认分段输出：每个 hop 一行（zone / server / IP / 耗时 / 类型 / 详情），
// 底部一行总耗时统计。
func FormatText(res *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — tracing from root (type %s)\n\n", res.Domain, res.QueryType)

	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	for _, hop := range res.Hops {
		switch hop.Type {
		case HopReferral:
			fmt.Fprintf(w, "%s\t%s\t%s\t%dms\treferral → %s\n",
				hop.Zone, hop.ServerName, hop.ServerIP, hop.QueryTimeMs, hop.ReferralZone)
		case HopAnswer:
			detail := strings.Join(hop.Answers, " / ")
			fmt.Fprintf(w, "%s\t%s\t%s\t%dms\t%s %s\n",
				hop.Zone, hop.ServerName, hop.ServerIP, hop.QueryTimeMs, res.QueryType, detail)
		case HopError:
			fmt.Fprintf(w, "%s\t%s\t%s\t%dms\t⚠ %s\n",
				hop.Zone, hop.ServerName, hop.ServerIP, hop.QueryTimeMs, hop.Error)
		}
	}
	_ = w.Flush()

	fmt.Fprintf(&b, "\ntotal %dms across %d hop", res.TotalTimeMs, len(res.Hops))
	if len(res.Hops) != 1 {
		fmt.Fprint(&b, "s")
	}
	fmt.Fprintln(&b)
	return b.String()
}

// FormatShort 输出仅最终答案（dig +short 风格）。无 final answer 时输出空。
func FormatShort(res *Result) string {
	if res.Final == nil {
		return ""
	}
	var b strings.Builder
	for _, v := range res.Final.Answers {
		fmt.Fprintln(&b, v)
	}
	return b.String()
}

// FormatVerbose 在默认 text 之上，每跳额外列出 NS referral 列表 + glue IPs。
func FormatVerbose(res *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — tracing from root (type %s)\n\n", res.Domain, res.QueryType)
	for i, hop := range res.Hops {
		fmt.Fprintf(&b, "[%d] %s\n", i, hop.Zone)
		fmt.Fprintf(&b, "    server: %s (%s)  rtt: %dms\n", hop.ServerName, hop.ServerIP, hop.QueryTimeMs)
		if hop.Rcode != "" {
			fmt.Fprintf(&b, "    rcode: %s\n", hop.Rcode)
		}
		switch hop.Type {
		case HopReferral:
			fmt.Fprintf(&b, "    referral → %s\n", hop.ReferralZone)
			for _, nsName := range hop.NSReferrals {
				if ips, ok := hop.GlueIPs[nsName]; ok {
					fmt.Fprintf(&b, "      NS %s → %s\n", nsName, strings.Join(ips, ", "))
				} else {
					fmt.Fprintf(&b, "      NS %s (glueless)\n", nsName)
				}
			}
		case HopAnswer:
			for _, v := range hop.Answers {
				fmt.Fprintf(&b, "    answer: %s\n", v)
			}
		case HopError:
			fmt.Fprintf(&b, "    error: %s\n", hop.Error)
		}
		fmt.Fprintln(&b)
	}
	fmt.Fprintf(&b, "total %dms across %d hops\n", res.TotalTimeMs, len(res.Hops))
	return b.String()
}

// FormatJSON 输出完整结构化结果。Hops/Final 通过 Go json tag 自动渲染。
func FormatJSON(res *Result) (string, error) {
	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Succeeded 返回 true 当 trace 拿到了 final answer。
func (r *Result) Succeeded() bool {
	return r.Final != nil
}

// HasAnyError 返回 true 当任一 hop 是 ERROR 类型。用于 --strict 模式 exit code。
func (r *Result) HasAnyError() bool {
	for _, h := range r.Hops {
		if h.Type == HopError {
			return true
		}
	}
	return false
}
