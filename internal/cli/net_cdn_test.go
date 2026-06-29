package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func runCDN(t *testing.T, deps cdnCmdDeps, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	deps.out = &out
	cmd := newCDNCommand(deps)
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute() // 先执行再读 buffer（求值顺序坑）
	return out.String(), err
}

// 三个采集函数都给桩，杜绝测试里走真实网络。
func stubDeps(headers map[string]string, ns []string, ips []netip.Addr, fetchErr error) cdnCmdDeps {
	return cdnCmdDeps{
		fetchHeaders: func(_ context.Context, _ string, _ bool, _ int, _ time.Duration) (string, map[string]string, error) {
			if fetchErr != nil {
				return "", nil, fetchErr
			}
			return "https://x.com/", headers, nil
		},
		lookupNS:  func(_ context.Context, _ string) ([]string, error) { return ns, nil },
		lookupIPs: func(_ context.Context, _ string) ([]netip.Addr, error) { return ips, nil },
	}
}

func TestCDN_DetectCloudflare(t *testing.T) {
	deps := stubDeps(
		map[string]string{"cf-ray": "8a1f-SJC", "server": "cloudflare"},
		[]string{"kim.ns.cloudflare.com"},
		[]netip.Addr{netip.MustParseAddr("104.16.1.1")},
		nil)
	out, err := runCDN(t, deps, "x.com")
	if err != nil {
		t.Fatalf("检测到 CDN 应退出 0：%v", err)
	}
	if !strings.Contains(out, "Cloudflare") || !strings.Contains(out, "SJC") {
		t.Errorf("应识别 Cloudflare + colo SJC，got:\n%s", out)
	}
}

func TestCDN_NotDetected_ReturnsError(t *testing.T) {
	deps := stubDeps(
		map[string]string{"server": "nginx"},
		[]string{"ns1.example.com"},
		[]netip.Addr{netip.MustParseAddr("93.184.216.34")},
		nil)
	out, err := runCDN(t, deps, "x.com")
	if err == nil {
		t.Error("没检测到 CDN（文本模式）应返回非 nil error，让退出码非 0")
	}
	if !strings.Contains(out, "没看到") {
		t.Errorf("应打印 not-detected 结论，got:\n%s", out)
	}
}

func TestCDN_JSON_AlwaysExitsZero(t *testing.T) {
	deps := stubDeps(
		map[string]string{"server": "nginx"},
		[]string{"ns1.example.com"},
		[]netip.Addr{netip.MustParseAddr("93.184.216.34")},
		nil)
	out, err := runCDN(t, deps, "x.com", "--json")
	if err != nil {
		t.Errorf("JSON 模式即使没检测到也应退出 0，got err=%v", err)
	}
	if !strings.Contains(out, `"detected": false`) {
		t.Errorf("JSON 应含 detected:false，got:\n%s", out)
	}
}

func TestCDN_HeadersOnly_SkipsDNS(t *testing.T) {
	dnsCalled := false
	deps := cdnCmdDeps{
		fetchHeaders: func(_ context.Context, _ string, _ bool, _ int, _ time.Duration) (string, map[string]string, error) {
			return "https://x.com/", map[string]string{"cf-ray": "abc-LHR"}, nil
		},
		lookupNS:  func(_ context.Context, _ string) ([]string, error) { dnsCalled = true; return nil, nil },
		lookupIPs: func(_ context.Context, _ string) ([]netip.Addr, error) { dnsCalled = true; return nil, nil },
	}
	out, err := runCDN(t, deps, "x.com", "--headers-only")
	if err != nil {
		t.Fatal(err)
	}
	if dnsCalled {
		t.Error("--headers-only 不该触发 DNS NS / IP 解析")
	}
	if !strings.Contains(out, "Cloudflare") {
		t.Errorf("仅凭 cf-ray 也应识别出 Cloudflare，got:\n%s", out)
	}
}

func TestCDN_FetchError(t *testing.T) {
	deps := stubDeps(nil, nil, nil, fmt.Errorf("dial tcp: connection refused"))
	_, err := runCDN(t, deps, "x.com")
	if err == nil {
		t.Error("HTTP 拉取失败应返回 error")
	}
}

func TestCDN_DNSFailureDegradesGracefully(t *testing.T) {
	// DNS 两路都失败，但响应头有 cf-ray → 仍应判出 Cloudflare、退出 0
	deps := cdnCmdDeps{
		fetchHeaders: func(_ context.Context, _ string, _ bool, _ int, _ time.Duration) (string, map[string]string, error) {
			return "https://x.com/", map[string]string{"cf-ray": "abc-FRA"}, nil
		},
		lookupNS:  func(_ context.Context, _ string) ([]string, error) { return nil, fmt.Errorf("no ns") },
		lookupIPs: func(_ context.Context, _ string) ([]netip.Addr, error) { return nil, fmt.Errorf("nxdomain") },
	}
	out, err := runCDN(t, deps, "x.com")
	if err != nil {
		t.Fatalf("DNS 失败应降级而非致命：%v", err)
	}
	if !strings.Contains(out, "Cloudflare") {
		t.Errorf("应仍凭响应头判出 Cloudflare，got:\n%s", out)
	}
}
