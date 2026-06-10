//go:build integration

// Trace integration 测试通过真实网络打 DNS。CI 默认不跑（无 -tags integration）。
//
// 本地运行：
//
//	go test -tags integration ./internal/dnstrace/...
//
// 这些测试容忍劫持环境：trace 的 hijack detection 会捕获被网关伪造的"假 ANSWER"
// 并标 ERROR，所以即使本地 DNS 拦截 UDP-53 流量，测试也能拿到 expected outcome
// （干净环境下成功，劫持环境下被 hijack detection 拦截）。
package dnstrace

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestIntegration_Trace_ExampleCom_OrFlagsHijack(t *testing.T) {
	tracer := NewTracer(Options{
		Bootstrap:    NewOSLookupResolver(3 * time.Second),
		HopTimeout:   3 * time.Second,
		TotalTimeout: 10 * time.Second,
	})
	res, err := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if len(res.Hops) == 0 {
		t.Fatal("expected at least 1 hop")
	}
	if res.Succeeded() {
		t.Logf("trace 干净环境下成功；final = %+v", res.Final)
	} else {
		last := res.Hops[len(res.Hops)-1]
		if last.Type != HopError {
			t.Errorf("失败时最后一跳应当是 ERROR，得到 %s", last.Type)
		}
		t.Logf("trace 失败（可能是劫持环境）：%s", last.Error)
	}
}

func TestIntegration_Trace_CustomServerSkipsHijackCheck(t *testing.T) {
	// --server 模式：直接打 1.1.1.1（recursive resolver），第一跳 ANSWER 合法
	tracer := NewTracer(Options{
		Bootstrap:    NewOSLookupResolver(3 * time.Second),
		HopTimeout:   3 * time.Second,
		TotalTimeout: 10 * time.Second,
		StartServer:  "1.1.1.1",
	})
	res, err := tracer.Trace(context.Background(), "example.com", dns.TypeA)
	if err != nil {
		t.Fatalf("Trace: %v", err)
	}
	if len(res.Hops) == 0 {
		t.Fatal("expected at least 1 hop")
	}
	if res.Hops[0].ServerName != "(custom)" {
		t.Errorf("--server 模式下应当显示 (custom)，得到 %q", res.Hops[0].ServerName)
	}
}
