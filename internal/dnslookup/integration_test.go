//go:build integration

// Integration 测试通过真实网络打 DNS。CI 默认不跑（无 -tags integration）。
//
// 本地运行：
//
//	go test -tags integration ./internal/dnslookup/...
//
// 测试目标固定指向 8.8.8.8 以避免本地 DNS 劫持/污染影响结果。
package dnslookup

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
)

const testServer = "8.8.8.8:53"

// detectHijack 探测当前网络是否劫持 DNS（NXDOMAIN 被替换为伪造 A 记录、
// 黑洞 IP 仍返回响应等）。劫持环境下与 NXDOMAIN/超时 行为相关的 integration
// 测试会失去意义，因此整体 Skip 而不是 Fail。
//
// 实现：查询 RFC 6761 保证不存在的 .invalid 域名，劫持时会返回 NOERROR + 假 A。
func detectHijack(t *testing.T) bool {
	t.Helper()
	r := NewResolver(3 * time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "jdan-hijack-probe-zzz999.invalid",
		Types:   []uint16{dns.TypeA},
		Server:  testServer,
		Timeout: 3 * time.Second,
	})
	if err != nil {
		return false
	}
	return res.Results[0].IsSuccess() && len(res.Results[0].Values) > 0
}

func TestIntegration_ExampleComBasicResolve(t *testing.T) {
	r := NewResolver(5 * time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "example.com",
		Types:   []uint16{dns.TypeA, dns.TypeNS},
		Server:  testServer,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !res.HasAnySuccess() {
		t.Fatalf("expected success, got: %+v", res.Results)
	}
	for _, tr := range res.Results {
		if tr.Type == "A" && len(tr.Values) == 0 {
			t.Error("expected example.com A record values")
		}
	}
}

func TestIntegration_DefaultSixTypes(t *testing.T) {
	r := NewResolver(5 * time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "google.com",
		Types:   DefaultTypes(),
		Server:  testServer,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(res.Results) != 6 {
		t.Fatalf("expected 6 results, got %d", len(res.Results))
	}
	if !res.HasAnySuccess() {
		t.Fatal("expected at least one success")
	}
	// google.com 至少有 A 和 NS
	hasA, hasNS := false, false
	for _, tr := range res.Results {
		if tr.Type == "A" && tr.IsSuccess() && len(tr.Values) > 0 {
			hasA = true
		}
		if tr.Type == "NS" && tr.IsSuccess() && len(tr.Values) > 0 {
			hasNS = true
		}
	}
	if !hasA || !hasNS {
		t.Errorf("expected A & NS for google.com, hasA=%v hasNS=%v", hasA, hasNS)
	}
}

func TestIntegration_NXDOMAIN(t *testing.T) {
	if detectHijack(t) {
		t.Skip("DNS 劫持环境（.invalid 域名被替换为伪造 A 记录），跳过 NXDOMAIN 测试")
	}
	// 用 .invalid TLD，RFC 6761 保证永不存在；干净环境下应返回 NXDOMAIN。
	r := NewResolver(5 * time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "nonexistent-jdan-test-12345.invalid",
		Types:   []uint16{dns.TypeA, dns.TypeAAAA},
		Server:  testServer,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !res.AllFailed() {
		t.Errorf("expected AllFailed for .invalid domain, got: %+v", res.Results)
	}
	if res.Results[0].Rcode != "NXDOMAIN" {
		t.Errorf("expected NXDOMAIN, got %q", res.Results[0].Rcode)
	}
}

func TestIntegration_TimeoutTriggers(t *testing.T) {
	if detectHijack(t) {
		t.Skip("DNS 劫持环境（黑洞 IP 的查询被网关拦截并伪造响应），跳过 timeout 测试")
	}
	// 黑洞 IP（TEST-NET-1, RFC 5737），干净环境下连接会一直 hang 直到超时。
	r := NewResolver(500 * time.Millisecond)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "example.com",
		Types:   []uint16{dns.TypeA},
		Server:  "192.0.2.1:53",
		Timeout: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if res.Results[0].Err != "TIMEOUT" {
		t.Errorf("expected TIMEOUT, got %+v", res.Results[0])
	}
}

// --- DoH integration tests ---

// 用 google / cloudflare 真实 DoH 服务器验证贯通。这些测试在劫持环境下应该
// 仍然成功，因为别名机制会绕过 OS resolver 直连 8.8.8.8 / 1.1.1.1。

func runDoHIntegration(t *testing.T, alias string) {
	t.Helper()
	target, err := ResolveDoHTarget(alias)
	if err != nil {
		t.Fatalf("ResolveDoHTarget(%q): %v", alias, err)
	}
	r := NewDoHResolver(target, 10*time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "example.com",
		Types:   []uint16{dns.TypeA, dns.TypeNS},
		Server:  target.URL,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !res.HasAnySuccess() {
		t.Fatalf("expected success via %s, got: %+v", alias, res.Results)
	}
}

func TestIntegration_DoH_Google(t *testing.T) {
	runDoHIntegration(t, "google")
}

func TestIntegration_DoH_Cloudflare(t *testing.T) {
	runDoHIntegration(t, "cloudflare")
}

func TestIntegration_DoH_BypassesLocalHijack(t *testing.T) {
	// 即使本地 DNS 把 .invalid 劫持成 fake A，DoH 应返回真 NXDOMAIN，
	// 因为别名机制直连 1.1.1.1 / 8.8.8.8 而非通过被劫持的本地 resolver。
	if !detectHijack(t) {
		t.Skip("非劫持环境，此测试不适用")
	}
	target, _ := ResolveDoHTarget("cloudflare")
	r := NewDoHResolver(target, 10*time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "nonexistent-jdan-doh-test-99999.invalid",
		Types:   []uint16{dns.TypeA},
		Server:  target.URL,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	// DoH 应该返回真实的 NXDOMAIN，不是被劫持的假 A
	if res.Results[0].Rcode != "NXDOMAIN" {
		t.Errorf("expected NXDOMAIN via DoH (bypass hijack), got rcode=%q values=%v",
			res.Results[0].Rcode, res.Results[0].Values)
	}
}

func TestIntegration_DoH_FullURLPath(t *testing.T) {
	// 自定义完整 URL 形式（无别名 bootstrap）。劫持环境下走 OS resolver 解析
	// dns.google，如果本地 DNS 也劫持 dns.google，TLS 会失败。所以仅在非劫持
	// 环境下测试这条 path。
	if detectHijack(t) {
		t.Skip("劫持环境下完整 URL 路径不可靠（DoH host 本身可能被劫持），跳过")
	}
	target, _ := ResolveDoHTarget("https://dns.google/dns-query")
	r := NewDoHResolver(target, 10*time.Second)
	res, err := Lookup(context.Background(), r, Options{
		Domain:  "example.com",
		Types:   []uint16{dns.TypeA},
		Server:  target.URL,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !res.HasAnySuccess() {
		t.Errorf("expected success via full-URL DoH, got: %+v", res.Results)
	}
}
