package dnstrace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// ------ Fake transport for unit tests ------

type fakeTransport struct {
	handler func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error)
}

func (f *fakeTransport) Exchange(ctx context.Context, msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
	// 尊重 ctx 超时（用于 hop/total timeout 测试）
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return f.handler(msg, server)
}

// ------ Helpers 构造 dns.Msg ------

func mkReferral(zone string, nsNames []string, glue map[string]string, ttl uint32) *dns.Msg {
	msg := new(dns.Msg)
	msg.Rcode = dns.RcodeSuccess
	for _, ns := range nsNames {
		msg.Ns = append(msg.Ns, &dns.NS{
			Hdr: dns.RR_Header{Name: zone, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
			Ns:  ns,
		})
	}
	for name, ip := range glue {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			continue
		}
		if v4 := parsed.To4(); v4 != nil {
			msg.Extra = append(msg.Extra, &dns.A{
				Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
				A:   v4,
			})
		} else {
			msg.Extra = append(msg.Extra, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
				AAAA: parsed,
			})
		}
	}
	return msg
}

func mkAnswerA(name, ip string, ttl uint32) *dns.Msg {
	msg := new(dns.Msg)
	msg.Rcode = dns.RcodeSuccess
	msg.Answer = append(msg.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
		A:   net.ParseIP(ip).To4(),
	})
	return msg
}

func mkAnswerNS(name string, nsNames []string, ttl uint32) *dns.Msg {
	msg := new(dns.Msg)
	msg.Rcode = dns.RcodeSuccess
	for _, ns := range nsNames {
		msg.Answer = append(msg.Answer, &dns.NS{
			Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
			Ns:  ns,
		})
	}
	return msg
}

func hostOf(server string) string {
	host, _, _ := net.SplitHostPort(server)
	return host
}

// ------ Core tests ------

func TestTrace_HappyPath_ThreeHops(t *testing.T) {
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		switch hostOf(server) {
		case "198.41.0.4": // a.root-servers.net
			return mkReferral("com.",
				[]string{"a.gtld-servers.net."},
				map[string]string{"a.gtld-servers.net.": "192.5.6.30"},
				172800), 5 * time.Millisecond, nil
		case "192.5.6.30":
			return mkReferral("example.com.",
				[]string{"a.iana-servers.net."},
				map[string]string{"a.iana-servers.net.": "199.43.135.53"},
				172800), 12 * time.Millisecond, nil
		case "199.43.135.53":
			return mkAnswerA("example.com.", "93.184.216.34", 3600), 45 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected server: %s", server)
	}}

	tracer := newTracerWithTransport(Options{}, transport)
	res, err := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("Trace error: %v", err)
	}
	if len(res.Hops) != 3 {
		t.Fatalf("expected 3 hops, got %d", len(res.Hops))
	}
	if res.Hops[0].Type != HopReferral || res.Hops[0].ReferralZone != "com." {
		t.Errorf("hop 0 wrong: %+v", res.Hops[0])
	}
	if res.Hops[1].Type != HopReferral || res.Hops[1].ReferralZone != "example.com." {
		t.Errorf("hop 1 wrong: %+v", res.Hops[1])
	}
	if res.Hops[2].Type != HopAnswer {
		t.Errorf("hop 2 should be ANSWER, got %s", res.Hops[2].Type)
	}
	if res.Final == nil || len(res.Final.Answers) == 0 || res.Final.Answers[0] != "93.184.216.34" {
		t.Errorf("Final answer wrong: %+v", res.Final)
	}
	if !res.Succeeded() {
		t.Error("expected Succeeded()=true")
	}
}

func TestTrace_GluelessNS_CallsBootstrap(t *testing.T) {
	// referral 给出 NS 但**不**提供 glue → 触发 bootstrap resolver 路径
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		switch hostOf(server) {
		case "198.41.0.4":
			// referral 到 com.，提供 glue
			return mkReferral("com.",
				[]string{"a.gtld-servers.net."},
				map[string]string{"a.gtld-servers.net.": "192.5.6.30"},
				172800), 5 * time.Millisecond, nil
		case "192.5.6.30":
			// referral 到 example.com.，**不提供 glue**（强制 glueless）
			return mkReferral("example.com.",
				[]string{"ns1.somethirdparty.net."},
				nil, 172800), 12 * time.Millisecond, nil
		case "8.8.8.9": // bootstrap 返回的 IP
			return mkAnswerA("example.com.", "1.2.3.4", 60), 8 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected server: %s", server)
	}}

	// fake bootstrap：把 ns1.somethirdparty.net 解析为 8.8.8.9
	bootstrap := &fakeBootstrap{
		responses: map[string]string{
			"ns1.somethirdparty.net.": "8.8.8.9",
		},
	}

	tracer := newTracerWithTransport(Options{Bootstrap: bootstrap}, transport)
	res, err := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if !res.Succeeded() {
		t.Fatalf("expected success, hops: %+v", res.Hops)
	}
	if bootstrap.calls == 0 {
		t.Error("bootstrap was not called for glueless NS")
	}
}

func TestTrace_GluelessNS_NoBootstrap_TerminatesWithError(t *testing.T) {
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		switch hostOf(server) {
		case "198.41.0.4":
			return mkReferral("com.",
				[]string{"a.gtld-servers.net."},
				map[string]string{"a.gtld-servers.net.": "192.5.6.30"}, 172800), 5 * time.Millisecond, nil
		case "192.5.6.30":
			return mkReferral("example.com.",
				[]string{"ns1.glueless.net."},
				nil, 172800), 12 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected server: %s", server)
	}}

	tracer := newTracerWithTransport(Options{Bootstrap: nil}, transport)
	res, _ := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if res.Succeeded() {
		t.Error("should not succeed with glueless + no bootstrap")
	}
	last := res.Hops[len(res.Hops)-1]
	if last.Type != HopError || !strings.Contains(last.Error, "glueless") {
		t.Errorf("expected glueless error, got: %+v", last)
	}
}

func TestTrace_CycleDetection(t *testing.T) {
	// root referral 到 a.evil.net.；a.evil.net referral 回 a.evil.net.（自环）
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		switch hostOf(server) {
		case "198.41.0.4":
			return mkReferral("evil.",
				[]string{"a.evil-ns.net."},
				map[string]string{"a.evil-ns.net.": "1.2.3.4"}, 60), 5 * time.Millisecond, nil
		case "1.2.3.4":
			// 这里 a.evil-ns.net 给出 referral 回到 evil.（已访问的 zone）
			return mkReferral("evil.",
				[]string{"a.evil-ns.net."},
				map[string]string{"a.evil-ns.net.": "1.2.3.4"}, 60), 5 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected: %s", server)
	}}

	tracer := newTracerWithTransport(Options{}, transport)
	res, _ := tracer.Trace(context.Background(), "anything.evil", dns.TypeA)
	if res.Succeeded() {
		t.Error("cycle should not lead to success")
	}
	last := res.Hops[len(res.Hops)-1]
	if !strings.Contains(last.Error, "cycle") {
		t.Errorf("expected cycle error, got: %+v", last)
	}
}

func TestTrace_MaxHopsExceeded(t *testing.T) {
	// 永远 referral 到一个 zone-N，N 递增（不会触发 cycle，会撞 max hops）
	zoneFor := func(n int) string { return fmt.Sprintf("z%d.", n) }
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		// 用 IP 编码当前 zone 深度
		switch hostOf(server) {
		case "198.41.0.4":
			return mkReferral(zoneFor(1), []string{"ns.z1."}, map[string]string{"ns.z1.": "10.0.0.1"}, 60), 1 * time.Millisecond, nil
		}
		host := hostOf(server)
		var n int
		fmt.Sscanf(host, "10.0.0.%d", &n)
		if n > 0 {
			return mkReferral(zoneFor(n+1), []string{fmt.Sprintf("ns.z%d.", n+1)}, map[string]string{fmt.Sprintf("ns.z%d.", n+1): fmt.Sprintf("10.0.0.%d", n+1)}, 60), 1 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected: %s", server)
	}}

	tracer := newTracerWithTransport(Options{MaxHops: 5}, transport)
	res, _ := tracer.Trace(context.Background(), "anything.z99", dns.TypeA)
	if res.Succeeded() {
		t.Error("should not succeed when max hops exhausted")
	}
	// hops 数 = MaxHops + 1 个 ERROR hop
	if len(res.Hops) != 6 {
		t.Errorf("expected 6 hops (5 referrals + 1 error), got %d", len(res.Hops))
	}
	last := res.Hops[len(res.Hops)-1]
	if !strings.Contains(last.Error, "max hops") {
		t.Errorf("expected max hops error, got: %v", last.Error)
	}
}

func TestTrace_SequentialNSFallback(t *testing.T) {
	// root server a 失败，b 给 referral，再走一跳到 answer。
	// 注意：root server 不能直接给 ANSWER 否则被 hijack detection 拦截。
	callCount := 0
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		callCount++
		switch hostOf(server) {
		case "198.41.0.4": // a 失败
			return nil, 1 * time.Millisecond, errors.New("i/o timeout")
		case "170.247.170.2": // b 给 referral
			return mkReferral("x.",
				[]string{"ns1.x."},
				map[string]string{"ns1.x.": "9.9.9.9"}, 60), 2 * time.Millisecond, nil
		case "9.9.9.9": // ns1.x 给最终 answer
			return mkAnswerA("x.", "1.2.3.4", 60), 1 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected: %s", server)
	}}

	tracer := newTracerWithTransport(Options{}, transport)
	res, _ := tracer.Trace(context.Background(), "x", dns.TypeA)
	if !res.Succeeded() {
		t.Fatalf("expected success after fallback, got: %+v", res.Hops)
	}
	if callCount != 3 {
		t.Errorf("expected 3 transport calls (a fail + b referral + ns answer), got %d", callCount)
	}
}

func TestTrace_AllNSFail_Terminates(t *testing.T) {
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		return nil, 0, errors.New("i/o timeout")
	}}

	tracer := newTracerWithTransport(Options{}, transport)
	res, _ := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if res.Succeeded() {
		t.Error("should not succeed when all root NSs fail")
	}
	last := res.Hops[len(res.Hops)-1]
	if last.Type != HopError || last.Error != "TIMEOUT" {
		t.Errorf("expected TIMEOUT, got %+v", last)
	}
}

func TestTrace_TotalTimeoutTriggersCtxCancel(t *testing.T) {
	// transport 慢响应，total timeout 短 → 第一次 hop 就被 ctx 取消
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		// 不睡眠——ctx 已经在 querySomeNS 内被 hopTimeout 包裹了
		// 这里直接返回 ctx.Err
		return nil, 0, context.DeadlineExceeded
	}}

	tracer := newTracerWithTransport(Options{TotalTimeout: 1 * time.Millisecond}, transport)
	res, _ := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if res.Succeeded() {
		t.Error("should not succeed with extreme timeout")
	}
}

func TestTrace_StartServerOverride(t *testing.T) {
	// --server 1.1.1.1 时，trace 从那台 server 起步
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		if hostOf(server) != "1.1.1.1" {
			t.Errorf("expected start server 1.1.1.1, got %s", server)
		}
		return mkAnswerA("x.", "9.9.9.9", 60), 1 * time.Millisecond, nil
	}}

	tracer := newTracerWithTransport(Options{StartServer: "1.1.1.1"}, transport)
	res, _ := tracer.Trace(context.Background(), "x", dns.TypeA)
	if !res.Succeeded() {
		t.Fatal("should succeed")
	}
	if res.Hops[0].ServerName != "(custom)" {
		t.Errorf("expected '(custom)' server name, got %q", res.Hops[0].ServerName)
	}
}

func TestTrace_TypeNSOverride(t *testing.T) {
	// --type NS 时最终一跳应当查 NS RR
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		switch hostOf(server) {
		case "198.41.0.4":
			return mkReferral("com.", []string{"a.gtld."}, map[string]string{"a.gtld.": "192.5.6.30"}, 60), 1 * time.Millisecond, nil
		case "192.5.6.30":
			// 注意：query type 是 NS。auth server 直接给 NS answer。
			return mkAnswerNS("example.com.", []string{"a.iana.", "b.iana."}, 60), 5 * time.Millisecond, nil
		}
		return nil, 0, fmt.Errorf("unexpected: %s", server)
	}}

	tracer := newTracerWithTransport(Options{}, transport)
	res, _ := tracer.Trace(context.Background(), "example.com", dns.TypeNS)
	if !res.Succeeded() {
		t.Fatalf("expected success, hops: %+v", res.Hops)
	}
	if len(res.Final.Answers) != 2 {
		t.Errorf("expected 2 NS answers, got %v", res.Final.Answers)
	}
}

func TestTrace_DetectsHijackedRootResponse(t *testing.T) {
	// 网关劫持场景：UDP-53 到 root server IP 被拦截，返回伪造的 ANSWER
	// 而不是 REFERRAL。trace 应当识别这是协议违规并报错。
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		// 任意 server 都返回 ANSWER（劫持网关的常见行为）
		return mkAnswerA("example.com.", "198.18.0.19", 1), 1 * time.Millisecond, nil
	}}

	tracer := newTracerWithTransport(Options{}, transport)
	res, _ := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if res.Succeeded() {
		t.Fatal("劫持的 ANSWER 不应被认作成功")
	}
	// 最后一跳应当是 ERROR 类型，包含"可疑"或"协议"字样
	last := res.Hops[len(res.Hops)-1]
	if last.Type != HopError {
		t.Errorf("expected hijack to be flagged as ERROR, got %s", last.Type)
	}
	if !strings.Contains(last.Error, "可疑") && !strings.Contains(last.Error, "协议") {
		t.Errorf("expected hijack hint, got: %s", last.Error)
	}
}

func TestTrace_StartServerSkipsHijackDetection(t *testing.T) {
	// 用户传 --server 覆盖时，可能是指向 recursive resolver（如 1.1.1.1）。
	// 这种场景下第一跳就拿 ANSWER 是合法的，不应触发 hijack detection。
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		return mkAnswerA("example.com.", "1.2.3.4", 60), 1 * time.Millisecond, nil
	}}

	tracer := newTracerWithTransport(Options{StartServer: "1.1.1.1"}, transport)
	res, _ := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if !res.Succeeded() {
		t.Errorf("StartServer 模式下首跳 ANSWER 应当被接受为合法，got: %+v", res.Hops)
	}
}

func TestTrace_HasAnyError(t *testing.T) {
	transport := &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		return nil, 0, errors.New("i/o timeout")
	}}
	tracer := newTracerWithTransport(Options{}, transport)
	res, _ := tracer.Trace(context.Background(), "x", dns.TypeA)
	if !res.HasAnyError() {
		t.Error("expected HasAnyError true")
	}
}

func TestTrace_RejectsEmptyDomain(t *testing.T) {
	tracer := newTracerWithTransport(Options{}, &fakeTransport{handler: func(msg *dns.Msg, server string) (*dns.Msg, time.Duration, error) {
		return nil, 0, nil
	}})
	if _, err := tracer.Trace(context.Background(), "", dns.TypeA); err == nil {
		t.Error("expected error for empty domain")
	}
}

// ------ fakeBootstrap：实现 dnslookup.Resolver ------

type fakeBootstrap struct {
	responses map[string]string // hostname → A IP
	calls     int
}

func (b *fakeBootstrap) Query(ctx context.Context, domain string, qtype uint16, _ string) (*dns.Msg, error) {
	b.calls++
	if qtype != dns.TypeA {
		return nil, fmt.Errorf("fake bootstrap only handles TypeA")
	}
	ip, ok := b.responses[domain]
	if !ok {
		return nil, fmt.Errorf("fake bootstrap: no entry for %s", domain)
	}
	return mkAnswerA(domain, ip, 60), nil
}

// ------ Format tests ------

func sampleResult() *Result {
	res := &Result{
		Domain:      "example.com.",
		QueryType:   "A",
		TotalTimeMs: 62,
		Hops: []Hop{
			{
				Zone: ".", ServerName: "a.root-servers.net.", ServerIP: "198.41.0.4",
				QueryTimeMs: 5, Type: HopReferral, Rcode: "NOERROR",
				ReferralZone: "com.",
				NSReferrals:  []string{"a.gtld-servers.net."},
				GlueIPs:      map[string][]string{"a.gtld-servers.net.": {"192.5.6.30"}},
			},
			{
				Zone: "com.", ServerName: "a.gtld-servers.net.", ServerIP: "192.5.6.30",
				QueryTimeMs: 12, Type: HopReferral, Rcode: "NOERROR",
				ReferralZone: "example.com.",
				NSReferrals:  []string{"a.iana-servers.net."},
				GlueIPs:      map[string][]string{"a.iana-servers.net.": {"199.43.135.53"}},
			},
			{
				Zone: "example.com.", ServerName: "a.iana-servers.net.", ServerIP: "199.43.135.53",
				QueryTimeMs: 45, Type: HopAnswer, Rcode: "NOERROR",
				Answers: []string{"93.184.216.34"},
			},
		},
	}
	final := res.Hops[2]
	res.Final = &final
	return res
}

func TestFormatText_ContainsHopsAndSummary(t *testing.T) {
	out := FormatText(sampleResult())
	if !strings.Contains(out, "example.com.") {
		t.Errorf("missing domain in output:\n%s", out)
	}
	if !strings.Contains(out, "a.root-servers.net.") {
		t.Errorf("missing root server in output:\n%s", out)
	}
	if !strings.Contains(out, "referral → com.") {
		t.Errorf("missing referral arrow:\n%s", out)
	}
	if !strings.Contains(out, "A 93.184.216.34") {
		t.Errorf("missing final answer:\n%s", out)
	}
	if !strings.Contains(out, "total 62ms across 3 hops") {
		t.Errorf("missing total summary:\n%s", out)
	}
}

func TestFormatShort_OnlyFinalAnswer(t *testing.T) {
	out := FormatShort(sampleResult())
	if strings.TrimSpace(out) != "93.184.216.34" {
		t.Errorf("expected only IP, got: %q", out)
	}
}

func TestFormatShort_EmptyWhenNoFinal(t *testing.T) {
	res := &Result{Hops: []Hop{{Type: HopError, Error: "TIMEOUT"}}}
	out := FormatShort(res)
	if out != "" {
		t.Errorf("expected empty, got: %q", out)
	}
}

func TestFormatVerbose_ContainsGlueDetail(t *testing.T) {
	out := FormatVerbose(sampleResult())
	if !strings.Contains(out, "NS a.gtld-servers.net. → 192.5.6.30") {
		t.Errorf("verbose should show NS → glue, got:\n%s", out)
	}
	if !strings.Contains(out, "rcode: NOERROR") {
		t.Errorf("verbose should show rcode:\n%s", out)
	}
}

func TestFormatJSON_ValidAndComplete(t *testing.T) {
	out, err := FormatJSON(sampleResult())
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}
	for _, want := range []string{
		`"domain"`, `"query_type"`, `"total_time_ms"`, `"hops"`, `"final"`,
		`"REFERRAL"`, `"ANSWER"`, `"93.184.216.34"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON missing %q in:\n%s", want, out)
		}
	}
}

// ------ friendly error ------

func TestFriendlyTraceErr(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"read udp 1.2.3.4:53: i/o timeout", "TIMEOUT"},
		{"context deadline exceeded", "TIMEOUT"},
		{"dial tcp: connection refused", "CONNECTION_REFUSED"},
		{"no route to host", "NO_ROUTE"},
		{"network is unreachable", "NETWORK_UNREACHABLE"},
		{"some other weird error", "some other weird error"},
	}
	for _, c := range cases {
		got := friendlyTraceErr(errors.New(c.in))
		if got != c.want {
			t.Errorf("friendlyTraceErr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ------ joinHostPort ------

func TestJoinHostPort(t *testing.T) {
	cases := []struct {
		host, port, want string
	}{
		{"8.8.8.8", "53", "8.8.8.8:53"},
		{"2001:db8::1", "53", "[2001:db8::1]:53"},
		{"dns.google", "53", "dns.google:53"},
	}
	for _, c := range cases {
		if got := joinHostPort(c.host, c.port); got != c.want {
			t.Errorf("joinHostPort(%q, %q) = %q, want %q", c.host, c.port, got, c.want)
		}
	}
}
