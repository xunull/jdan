package ipx

import (
	"net/netip"
	"strings"
	"testing"
)

func cidrs(t *testing.T, ss ...string) []netip.Prefix {
	t.Helper()
	out := make([]netip.Prefix, len(ss))
	for i, s := range ss {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			t.Fatalf("bad test prefix %q: %v", s, err)
		}
		out[i] = p
	}
	return out
}

func joinPrefixes(ps []netip.Prefix) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = p.String()
	}
	return strings.Join(parts, " ")
}

func TestRangeToCIDRs_Classic(t *testing.T) {
	start := netip.MustParseAddr("192.168.1.5")
	end := netip.MustParseAddr("192.168.1.20")
	got, err := RangeToCIDRs(start, end)
	if err != nil {
		t.Fatal(err)
	}
	want := "192.168.1.5/32 192.168.1.6/31 192.168.1.8/29 192.168.1.16/30 192.168.1.20/32"
	if joinPrefixes(got) != want {
		t.Errorf("range→cidr =\n  %s\nwant\n  %s", joinPrefixes(got), want)
	}
}

func TestRangeToCIDRs_ExactBlock(t *testing.T) {
	// 整块对齐 → 单个 CIDR
	got, err := RangeToCIDRs(netip.MustParseAddr("10.0.0.0"), netip.MustParseAddr("10.0.0.255"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "10.0.0.0/24" {
		t.Errorf("= %s, want 10.0.0.0/24", joinPrefixes(got))
	}
}

func TestRangeToCIDRs_SingleAddr(t *testing.T) {
	got, err := RangeToCIDRs(netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("1.2.3.4"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "1.2.3.4/32" {
		t.Errorf("= %s, want 1.2.3.4/32", joinPrefixes(got))
	}
}

func TestRangeToCIDRs_V6(t *testing.T) {
	got, err := RangeToCIDRs(netip.MustParseAddr("2001:db8::"), netip.MustParseAddr("2001:db8::ff"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "2001:db8::/120" {
		t.Errorf("= %s, want 2001:db8::/120", joinPrefixes(got))
	}
}

func TestRangeToCIDRs_Errors(t *testing.T) {
	if _, err := RangeToCIDRs(netip.MustParseAddr("10.0.0.10"), netip.MustParseAddr("10.0.0.1")); err == nil {
		t.Error("start>end 应报错")
	}
	if _, err := RangeToCIDRs(netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("2001:db8::1")); err == nil {
		t.Error("跨族应报错")
	}
}

func TestAggregate_MergeAdjacent(t *testing.T) {
	got, err := Aggregate(cidrs(t, "10.0.0.0/25", "10.0.0.128/25"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "10.0.0.0/24" {
		t.Errorf("= %s, want 10.0.0.0/24", joinPrefixes(got))
	}
}

func TestAggregate_Contained(t *testing.T) {
	// /25 完全被 /24 包含 → 只留 /24
	got, err := Aggregate(cidrs(t, "10.0.0.0/24", "10.0.0.128/25"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "10.0.0.0/24" {
		t.Errorf("= %s, want 10.0.0.0/24", joinPrefixes(got))
	}
}

func TestAggregate_Gap(t *testing.T) {
	// 中间有洞（10.0.1.x 缺）→ 不能合并
	got, err := Aggregate(cidrs(t, "10.0.2.0/24", "10.0.0.0/24"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "10.0.0.0/24 10.0.2.0/24" {
		t.Errorf("= %s, want 10.0.0.0/24 10.0.2.0/24", joinPrefixes(got))
	}
}

func TestAggregate_MixedFamilies(t *testing.T) {
	// v4 聚合 + v6 聚合，各归各；结果先 v4 后 v6
	got, err := Aggregate(cidrs(t, "2001:db8:8000::/33", "10.0.0.0/25", "2001:db8::/33", "10.0.0.128/25"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "10.0.0.0/24 2001:db8::/32" {
		t.Errorf("= %s, want 10.0.0.0/24 2001:db8::/32", joinPrefixes(got))
	}
}

func TestAggregate_HostBitsNormalized(t *testing.T) {
	// 带 host bit 的输入应先 Masked
	got, err := Aggregate(cidrs(t, "10.0.0.5/25", "10.0.0.200/25"))
	if err != nil {
		t.Fatal(err)
	}
	if joinPrefixes(got) != "10.0.0.0/24" {
		t.Errorf("= %s, want 10.0.0.0/24", joinPrefixes(got))
	}
}

func TestAggregate_Empty(t *testing.T) {
	if _, err := Aggregate(nil); err == nil {
		t.Error("空输入应报错")
	}
}
