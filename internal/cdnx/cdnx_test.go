package cdnx

import (
	"net/netip"
	"strings"
	"testing"
)

func TestDetect_CloudflareByRay(t *testing.T) {
	res := Detect(
		map[string]string{"cf-ray": "8a1f2c3d4e5f6789-SJC", "server": "cloudflare"},
		nil, nil, DefaultProviders())
	if !res.Detected() || res.Matches[0].Provider != "Cloudflare" {
		t.Fatalf("应识别出 Cloudflare，got %+v", res.Matches)
	}
	if res.Matches[0].Confidence != "确定" {
		t.Errorf("cf-ray 是强指纹，应为确定，got %q", res.Matches[0].Confidence)
	}
	if res.Colo != "SJC" {
		t.Errorf("应从 CF-RAY 解出 colo SJC，got %q", res.Colo)
	}
}

func TestDetect_CloudflareByIP(t *testing.T) {
	res := Detect(nil, nil,
		[]netip.Addr{netip.MustParseAddr("104.16.5.5")}, DefaultProviders())
	if !res.Detected() || res.Matches[0].Confidence != "确定" {
		t.Errorf("IP 落 CF 段是强证据，应确定 Cloudflare，got %+v", res.Matches)
	}
}

func TestDetect_NSOnlyIsLikely(t *testing.T) {
	res := Detect(nil,
		[]string{"kim.ns.cloudflare.com"}, nil, DefaultProviders())
	if !res.Detected() {
		t.Fatal("NS 指向 cloudflare 应命中")
	}
	if res.Matches[0].Confidence != "很可能" {
		t.Errorf("只有单路弱信号应为很可能，got %q", res.Matches[0].Confidence)
	}
}

func TestDetect_TwoWeakKindsBecomeCertain(t *testing.T) {
	// server: cloudflare（弱）+ NS（弱），两路一致 → 确定
	res := Detect(
		map[string]string{"server": "cloudflare"},
		[]string{"walt.ns.cloudflare.com"}, nil, DefaultProviders())
	if !res.Detected() || res.Matches[0].Confidence != "确定" {
		t.Errorf("两路弱信号一致应升为确定，got %+v", res.Matches)
	}
}

func TestDetect_CloudFront(t *testing.T) {
	res := Detect(
		map[string]string{"x-amz-cf-id": "abc==", "via": "1.1 abc.cloudfront.net"},
		nil, nil, DefaultProviders())
	if !res.Detected() || res.Matches[0].Provider != "Amazon CloudFront" {
		t.Errorf("应识别 CloudFront，got %+v", res.Matches)
	}
	if res.Matches[0].Confidence != "确定" {
		t.Errorf("x-amz-cf-id 是强指纹，应确定，got %q", res.Matches[0].Confidence)
	}
}

func TestDetect_None(t *testing.T) {
	res := Detect(
		map[string]string{"server": "nginx", "x-powered-by": "php"},
		[]string{"ns1.example.com", "ns2.example.com"},
		[]netip.Addr{netip.MustParseAddr("93.184.216.34")},
		DefaultProviders())
	if res.Detected() {
		t.Errorf("纯 nginx + 非 CDN NS/IP 不该命中，got %+v", res.Matches)
	}
}

func TestColoFromCFRay(t *testing.T) {
	cases := map[string]string{
		"8a1f2c3d4e5f6789-SJC": "SJC",
		"deadbeef-lhr":         "LHR", // 小写归一为大写
		"noColoHere":           "",    // 无 '-'
		"abc-12":               "",    // 不足 3 字母
		"abc-NRT5":             "",    // 含数字非纯字母
	}
	for in, want := range cases {
		if got := ColoFromCFRay(in); got != want {
			t.Errorf("ColoFromCFRay(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCloudflareRanges_AllParse(t *testing.T) {
	// 往返保证：每条内嵌 CIDR 都能解析（否则 cloudflareRanges 会少几条）
	if got, want := len(cloudflareRanges()), len(cloudflareCIDRs); got != want {
		t.Errorf("有 CIDR 解析失败：解析出 %d 条，内嵌 %d 条", got, want)
	}
}

func TestCloudflareRanges_Contains(t *testing.T) {
	ranges := cloudflareRanges()
	hit := func(s string) bool {
		a := netip.MustParseAddr(s)
		for _, r := range ranges {
			if r.Contains(a) {
				return true
			}
		}
		return false
	}
	if !hit("104.16.0.1") {
		t.Error("104.16.0.1 应在 CF v4 段内")
	}
	if !hit("2606:4700::1") {
		t.Error("2606:4700::1 应在 CF v6 段内")
	}
	if hit("8.8.8.8") {
		t.Error("8.8.8.8 不该被判进 CF 段")
	}
}

func TestFormatJSON(t *testing.T) {
	res := Detect(map[string]string{"cf-ray": "abc-SJC"}, nil, nil, DefaultProviders())
	s, err := FormatJSON(res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, `"detected": true`) || !strings.Contains(s, `"provider": "Cloudflare"`) {
		t.Errorf("JSON 应含 detected:true 与 provider:Cloudflare，got:\n%s", s)
	}
}
