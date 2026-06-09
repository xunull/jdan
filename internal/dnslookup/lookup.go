package dnslookup

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Options 控制一次 Lookup 调用的行为。
type Options struct {
	Domain  string
	Types   []uint16      // 至少 1 个 record type
	Server  string        // 不带端口时由 ensurePort 补 :53
	Timeout time.Duration // 整体查询超时；<=0 表示不另外加 timeout
}

// TypeResult 单个 type 的查询结果。
//
// 三种状态：
//   - 成功有记录：Err 为空，Rcode == "NOERROR"，Values 非空
//   - 成功空记录：Err 为空，Rcode == "NOERROR"，Values 为空（不是错误，只是该 type 没有 RR）
//   - 失败：Err 非空（网络/超时）或 Rcode != "NOERROR"（NXDOMAIN/SERVFAIL/...）
type TypeResult struct {
	Type   string   `json:"type"`
	Rcode  string   `json:"rcode,omitempty"`
	TTL    uint32   `json:"ttl,omitempty"`
	Values []string `json:"values"` // 始终非 nil，JSON 渲染为 [] 而非 null
	Err    string   `json:"error,omitempty"`
}

// IsSuccess 返回 true 当查询为 NOERROR（含空记录）。NXDOMAIN/SERVFAIL/TIMEOUT 等返回 false。
func (t TypeResult) IsSuccess() bool {
	return t.Err == "" && t.Rcode == "NOERROR"
}

// Result 一次 Lookup 调用的全量结果。
type Result struct {
	Domain      string       `json:"domain"`
	Server      string       `json:"server"`
	QueryTimeMs int64        `json:"query_time_ms"`
	Results     []TypeResult `json:"results"`
}

// HasAnySuccess 返回 true 当至少有一个 type 查询为 NOERROR。
func (r *Result) HasAnySuccess() bool {
	for _, t := range r.Results {
		if t.IsSuccess() {
			return true
		}
	}
	return false
}

// AllFailed 返回 true 当所有 type 都失败。决定宽容模式下的 exit code。
func (r *Result) AllFailed() bool {
	return !r.HasAnySuccess()
}

// AnyFailed 返回 true 当至少一个 type 失败。决定 --strict 模式下的 exit code。
func (r *Result) AnyFailed() bool {
	for _, t := range r.Results {
		if !t.IsSuccess() {
			return true
		}
	}
	return false
}

// Lookup 并发查询 opts.Types 里的每个 record type，返回合并后的 Result。
//
// Lookup 本身只在配置错误时返回 error（domain 空、type 列表空、server 空）；
// 单个 type 的网络/协议失败记录在对应 TypeResult.Err / .Rcode 中。
func Lookup(ctx context.Context, r Resolver, opts Options) (*Result, error) {
	if opts.Domain == "" {
		return nil, fmt.Errorf("domain 不能为空")
	}
	if len(opts.Types) == 0 {
		return nil, fmt.Errorf("至少需要 1 个 record type")
	}
	if opts.Server == "" {
		return nil, fmt.Errorf("server 不能为空")
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	server := ensurePort(opts.Server)
	res := &Result{
		Domain:  opts.Domain,
		Server:  server,
		Results: make([]TypeResult, len(opts.Types)),
	}

	start := time.Now()
	var wg sync.WaitGroup
	for i, qtype := range opts.Types {
		wg.Add(1)
		go func(i int, qtype uint16) {
			defer wg.Done()
			res.Results[i] = queryOneType(ctx, r, opts.Domain, qtype, server)
		}(i, qtype)
	}
	wg.Wait()
	res.QueryTimeMs = time.Since(start).Milliseconds()

	return res, nil
}

func queryOneType(ctx context.Context, r Resolver, domain string, qtype uint16, server string) TypeResult {
	typeStr := dns.TypeToString[qtype]
	if typeStr == "" {
		typeStr = fmt.Sprintf("TYPE%d", qtype)
	}
	out := TypeResult{Type: typeStr, Values: []string{}}

	msg, err := r.Query(ctx, domain, qtype, server)
	if err != nil {
		out.Err = friendlyErr(err)
		return out
	}
	if msg == nil {
		out.Err = "EMPTY_RESPONSE"
		return out
	}

	out.Rcode = dns.RcodeToString[msg.Rcode]
	if out.Rcode == "" {
		out.Rcode = fmt.Sprintf("RCODE%d", msg.Rcode)
	}

	if len(msg.Answer) == 0 {
		return out
	}

	const noTTL = ^uint32(0)
	minTTL := noTTL
	for _, rr := range msg.Answer {
		if rr.Header().Ttl < minTTL {
			minTTL = rr.Header().Ttl
		}
		if v := renderRR(rr); v != "" {
			out.Values = append(out.Values, v)
		}
	}
	if minTTL != noTTL {
		out.TTL = minTTL
	}
	return out
}

// renderRR 把单条 RR 渲染为面向用户的紧凑字符串（不含 type/ttl/class 元数据，因为这些在外层）。
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
	case *dns.SOA:
		return fmt.Sprintf("%s %s %d %d %d %d %d",
			v.Ns, v.Mbox, v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)
	case *dns.CAA:
		return fmt.Sprintf("%d %s %q", v.Flag, v.Tag, v.Value)
	case *dns.SRV:
		return fmt.Sprintf("%d %d %d %s", v.Priority, v.Weight, v.Port, v.Target)
	case *dns.PTR:
		return v.Ptr
	default:
		// 退路：用 miekg 默认 String() 去掉 "<owner>\t<ttl>\t<class>\t<type>\t" 前缀
		s := rr.String()
		// 找第 4 个 tab，之后是 rdata
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

// friendlyErr 把底层网络错误翻译为更人类化、便于在表格里展示的短标识。
func friendlyErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "i/o timeout"),
		strings.Contains(msg, "context deadline exceeded"),
		strings.Contains(msg, "context canceled"):
		return "TIMEOUT"
	case strings.Contains(msg, "connection refused"):
		return "CONNECTION_REFUSED"
	case strings.Contains(msg, "no such host"):
		return "NO_SUCH_HOST"
	default:
		return msg
	}
}

// DefaultTypes 是默认查询的 6 个 record type（plan D4 锁定）。
func DefaultTypes() []uint16 {
	return []uint16{
		dns.TypeA, dns.TypeAAAA, dns.TypeMX,
		dns.TypeTXT, dns.TypeCNAME, dns.TypeNS,
	}
}

// AllTypes 是 -t all 的 9 个 record type（默认 6 + SOA + CAA + SRV）。
func AllTypes() []uint16 {
	return []uint16{
		dns.TypeA, dns.TypeAAAA, dns.TypeMX,
		dns.TypeTXT, dns.TypeCNAME, dns.TypeNS,
		dns.TypeSOA, dns.TypeCAA, dns.TypeSRV,
	}
}

// ParseTypes 把 "" / "A" / "A,MX,TXT" / "all" 解析为 type 列表。
func ParseTypes(s string) ([]uint16, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultTypes(), nil
	}
	if strings.EqualFold(s, "all") {
		return AllTypes(), nil
	}
	parts := strings.Split(s, ",")
	out := make([]uint16, 0, len(parts))
	seen := make(map[uint16]struct{}, len(parts))
	for _, p := range parts {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		qt, ok := dns.StringToType[p]
		if !ok {
			return nil, fmt.Errorf("不支持的 record type: %q", p)
		}
		if _, dup := seen[qt]; dup {
			continue
		}
		seen[qt] = struct{}{}
		out = append(out, qt)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("type 列表为空")
	}
	return out, nil
}
