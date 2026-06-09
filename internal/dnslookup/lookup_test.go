package dnslookup

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/miekg/dns"
)

// fakeResolver 在测试中替换真实 miekgResolver。按 qtype 路由到预设响应。
type fakeResolver struct {
	results map[uint16]fakeResp
}

type fakeResp struct {
	msg   *dns.Msg
	err   error
	delay time.Duration
}

func (f *fakeResolver) Query(ctx context.Context, domain string, qtype uint16, server string) (*dns.Msg, error) {
	r, ok := f.results[qtype]
	if !ok {
		return nil, errors.New("fake: no preset response for type")
	}
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.msg, r.err
}

// helper：构造 NOERROR 响应 + 答案 RR。
func mkMsg(rcode int, answers ...dns.RR) *dns.Msg {
	m := new(dns.Msg)
	m.Rcode = rcode
	m.Answer = answers
	return m
}

func mkA(name string, ttl uint32, ip string) *dns.A {
	return &dns.A{
		Hdr: dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
		A:   net.ParseIP(ip).To4(),
	}
}

func mkAAAA(name string, ttl uint32, ip string) *dns.AAAA {
	return &dns.AAAA{
		Hdr:  dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
		AAAA: net.ParseIP(ip),
	}
}

func mkMX(name string, ttl uint32, pref uint16, mx string) *dns.MX {
	return &dns.MX{
		Hdr:        dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttl},
		Preference: pref,
		Mx:         dns.Fqdn(mx),
	}
}

func mkTXT(name string, ttl uint32, txt ...string) *dns.TXT {
	return &dns.TXT{
		Hdr: dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
		Txt: txt,
	}
}

func mkCNAME(name string, ttl uint32, target string) *dns.CNAME {
	return &dns.CNAME{
		Hdr:    dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
		Target: dns.Fqdn(target),
	}
}

func mkNS(name string, ttl uint32, ns string) *dns.NS {
	return &dns.NS{
		Hdr: dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
		Ns:  dns.Fqdn(ns),
	}
}

func baseOpts() Options {
	return Options{
		Domain:  "example.com",
		Server:  "8.8.8.8:53",
		Timeout: 2 * time.Second,
	}
}

func TestLookup_AllSuccess(t *testing.T) {
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeA:     {msg: mkMsg(dns.RcodeSuccess, mkA("example.com", 3600, "93.184.216.34"))},
		dns.TypeAAAA:  {msg: mkMsg(dns.RcodeSuccess, mkAAAA("example.com", 3600, "2606:2800:220:1:248:1893:25c8:1946"))},
		dns.TypeMX:    {msg: mkMsg(dns.RcodeSuccess, mkMX("example.com", 1800, 10, "mx.example.com"))},
		dns.TypeTXT:   {msg: mkMsg(dns.RcodeSuccess, mkTXT("example.com", 600, "v=spf1 -all"))},
		dns.TypeCNAME: {msg: mkMsg(dns.RcodeSuccess)},
		dns.TypeNS:    {msg: mkMsg(dns.RcodeSuccess, mkNS("example.com", 86400, "a.iana-servers.net"), mkNS("example.com", 86400, "b.iana-servers.net"))},
	}}
	opts := baseOpts()
	opts.Types = DefaultTypes()
	res, err := Lookup(context.Background(), fake, opts)
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if len(res.Results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(res.Results))
	}
	if !res.HasAnySuccess() {
		t.Error("expected HasAnySuccess true")
	}
	if res.AnyFailed() {
		t.Error("expected AnyFailed false")
	}
	if res.Results[0].Type != "A" || res.Results[0].TTL != 3600 || len(res.Results[0].Values) != 1 {
		t.Errorf("A result wrong: %+v", res.Results[0])
	}
}

func TestLookup_PartialTimeout(t *testing.T) {
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeA:    {msg: mkMsg(dns.RcodeSuccess, mkA("example.com", 100, "1.2.3.4"))},
		dns.TypeAAAA: {err: errors.New("read udp: i/o timeout")},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeA, dns.TypeAAAA}
	res, _ := Lookup(context.Background(), fake, opts)
	if !res.HasAnySuccess() {
		t.Error("partial success should mark HasAnySuccess true")
	}
	if !res.AnyFailed() {
		t.Error("partial failure should mark AnyFailed true")
	}
	if res.AllFailed() {
		t.Error("AllFailed should be false")
	}
	if res.Results[1].Err != "TIMEOUT" {
		t.Errorf("expected TIMEOUT, got %q", res.Results[1].Err)
	}
}

func TestLookup_AllNXDOMAIN(t *testing.T) {
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeA:    {msg: mkMsg(dns.RcodeNameError)},
		dns.TypeAAAA: {msg: mkMsg(dns.RcodeNameError)},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeA, dns.TypeAAAA}
	res, _ := Lookup(context.Background(), fake, opts)
	if !res.AllFailed() {
		t.Error("all NXDOMAIN should mark AllFailed true")
	}
	if res.Results[0].Rcode != "NXDOMAIN" {
		t.Errorf("expected NXDOMAIN, got %q", res.Results[0].Rcode)
	}
}

func TestLookup_EmptyAnswerStillSuccess(t *testing.T) {
	// 域名存在但该 type 无记录（如域名没设 MX）：rcode NOERROR + 空 Answer。
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeMX: {msg: mkMsg(dns.RcodeSuccess)},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeMX}
	res, _ := Lookup(context.Background(), fake, opts)
	r := res.Results[0]
	if !r.IsSuccess() {
		t.Errorf("empty NOERROR should be success, got %+v", r)
	}
	if len(r.Values) != 0 {
		t.Errorf("expected 0 values, got %d", len(r.Values))
	}
	if r.TTL != 0 {
		t.Errorf("expected TTL 0 for empty answer, got %d", r.TTL)
	}
}

func TestLookup_TTLIsMinimum(t *testing.T) {
	// 同 type 多 RR 的 TTL 取最小（与 dig 行为一致）。
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeA: {msg: mkMsg(dns.RcodeSuccess,
			mkA("example.com", 3600, "1.1.1.1"),
			mkA("example.com", 600, "2.2.2.2"),
			mkA("example.com", 1800, "3.3.3.3"),
		)},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeA}
	res, _ := Lookup(context.Background(), fake, opts)
	if res.Results[0].TTL != 600 {
		t.Errorf("expected min TTL 600, got %d", res.Results[0].TTL)
	}
	if len(res.Results[0].Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(res.Results[0].Values))
	}
}

func TestLookup_TXTQuoting(t *testing.T) {
	// TXT 多 string + 含双引号要正确转义。
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeTXT: {msg: mkMsg(dns.RcodeSuccess,
			mkTXT("example.com", 600, `v=spf1 include:_spf.google.com -all`),
			mkTXT("example.com", 600, "fragment1", `with "quotes"`),
		)},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeTXT}
	res, _ := Lookup(context.Background(), fake, opts)
	if len(res.Results[0].Values) != 2 {
		t.Fatalf("expected 2 TXT values, got %d", len(res.Results[0].Values))
	}
	if res.Results[0].Values[0] != `"v=spf1 include:_spf.google.com -all"` {
		t.Errorf("TXT[0] wrong: %q", res.Results[0].Values[0])
	}
	if res.Results[0].Values[1] != `"fragment1" "with \"quotes\""` {
		t.Errorf("TXT[1] wrong: %q", res.Results[0].Values[1])
	}
}

func TestLookup_MXRendering(t *testing.T) {
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeMX: {msg: mkMsg(dns.RcodeSuccess,
			mkMX("example.com", 1800, 10, "mx1.example.com"),
			mkMX("example.com", 1800, 20, "mx2.example.com"),
		)},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeMX}
	res, _ := Lookup(context.Background(), fake, opts)
	want := []string{"10 mx1.example.com.", "20 mx2.example.com."}
	if !reflect.DeepEqual(res.Results[0].Values, want) {
		t.Errorf("MX rendering wrong: got %v, want %v", res.Results[0].Values, want)
	}
}

func TestLookup_CtxTimeoutPropagates(t *testing.T) {
	// 整体 timeout 触发后，未完成的 type 标 TIMEOUT，已完成的保留。
	fake := &fakeResolver{results: map[uint16]fakeResp{
		dns.TypeA:    {msg: mkMsg(dns.RcodeSuccess, mkA("example.com", 100, "1.2.3.4"))},
		dns.TypeAAAA: {delay: 500 * time.Millisecond, msg: mkMsg(dns.RcodeSuccess)},
	}}
	opts := baseOpts()
	opts.Types = []uint16{dns.TypeA, dns.TypeAAAA}
	opts.Timeout = 50 * time.Millisecond

	res, err := Lookup(context.Background(), fake, opts)
	if err != nil {
		t.Fatalf("Lookup error: %v", err)
	}
	if res.Results[0].Err != "" || !res.Results[0].IsSuccess() {
		t.Errorf("fast A should succeed, got %+v", res.Results[0])
	}
	if res.Results[1].Err != "TIMEOUT" {
		t.Errorf("slow AAAA should be TIMEOUT, got %+v", res.Results[1])
	}
}

func TestLookup_RejectsBadOptions(t *testing.T) {
	fake := &fakeResolver{}
	cases := []struct {
		name string
		opts Options
	}{
		{"empty domain", Options{Server: "8.8.8.8:53", Types: []uint16{dns.TypeA}}},
		{"empty types", Options{Domain: "x", Server: "8.8.8.8:53"}},
		{"empty server", Options{Domain: "x", Types: []uint16{dns.TypeA}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := Lookup(context.Background(), fake, c.opts); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestParseTypes(t *testing.T) {
	cases := []struct {
		in   string
		want []uint16
		err  bool
	}{
		{"", DefaultTypes(), false},
		{"A", []uint16{dns.TypeA}, false},
		{"a", []uint16{dns.TypeA}, false},
		{"A,MX,TXT", []uint16{dns.TypeA, dns.TypeMX, dns.TypeTXT}, false},
		{" A , MX , TXT ", []uint16{dns.TypeA, dns.TypeMX, dns.TypeTXT}, false},
		{"A,A,MX", []uint16{dns.TypeA, dns.TypeMX}, false}, // 去重
		{"all", AllTypes(), false},
		{"ALL", AllTypes(), false},
		{"INVALID", nil, true},
		{"A,INVALID", nil, true},
		{",,,", nil, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseTypes(c.in)
			if c.err {
				if err == nil {
					t.Errorf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestTypeResult_IsSuccess(t *testing.T) {
	cases := []struct {
		name string
		r    TypeResult
		want bool
	}{
		{"noerror+values", TypeResult{Rcode: "NOERROR", Values: []string{"x"}}, true},
		{"noerror+empty", TypeResult{Rcode: "NOERROR", Values: []string{}}, true},
		{"nxdomain", TypeResult{Rcode: "NXDOMAIN"}, false},
		{"servfail", TypeResult{Rcode: "SERVFAIL"}, false},
		{"timeout err", TypeResult{Err: "TIMEOUT"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.r.IsSuccess(); got != c.want {
				t.Errorf("IsSuccess = %v, want %v", got, c.want)
			}
		})
	}
}
