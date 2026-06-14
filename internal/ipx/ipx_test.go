package ipx

import (
	"net/netip"
	"strings"
	"testing"
)

// ---- Classify ----

func TestClassify_IPv4Private(t *testing.T) {
	cases := []string{"10.5.1.2", "192.168.1.42", "172.16.0.1"}
	for _, s := range cases {
		c := Classify(netip.MustParseAddr(s))
		if !c.Private {
			t.Errorf("%s should be Private", s)
		}
	}
}

func TestClassify_IPv4Loopback(t *testing.T) {
	c := Classify(netip.MustParseAddr("127.0.0.1"))
	if !c.Loopback {
		t.Errorf("127.0.0.1 should be Loopback")
	}
}

func TestClassify_IPv4Multicast(t *testing.T) {
	c := Classify(netip.MustParseAddr("224.0.0.1"))
	if !c.Multicast {
		t.Errorf("224.0.0.1 should be Multicast")
	}
}

func TestClassify_IPv4LinkLocal(t *testing.T) {
	c := Classify(netip.MustParseAddr("169.254.1.1"))
	if !c.LinkLocal {
		t.Errorf("169.254.1.1 should be LinkLocal")
	}
}

func TestClassify_IPv4CGNAT(t *testing.T) {
	c := Classify(netip.MustParseAddr("100.64.0.1"))
	if !c.CGNAT {
		t.Errorf("100.64.0.1 should be CGNAT")
	}
}

func TestClassify_IPv4Documentation(t *testing.T) {
	for _, s := range []string{"192.0.2.1", "198.51.100.1", "203.0.113.1"} {
		c := Classify(netip.MustParseAddr(s))
		if !c.Documentation {
			t.Errorf("%s should be Documentation", s)
		}
	}
}

func TestClassify_IPv4GlobalUnicast(t *testing.T) {
	c := Classify(netip.MustParseAddr("8.8.8.8"))
	if !c.GlobalUnicast {
		t.Errorf("8.8.8.8 should be GlobalUnicast")
	}
	if c.Private || c.Loopback {
		t.Errorf("8.8.8.8 should not be Private/Loopback")
	}
}

func TestClassify_IPv6Loopback(t *testing.T) {
	c := Classify(netip.MustParseAddr("::1"))
	if !c.Loopback {
		t.Errorf("::1 should be Loopback")
	}
}

func TestClassify_IPv6LinkLocal(t *testing.T) {
	c := Classify(netip.MustParseAddr("fe80::1"))
	if !c.LinkLocal {
		t.Errorf("fe80::1 should be LinkLocal")
	}
}

func TestClassify_IPv6Documentation(t *testing.T) {
	c := Classify(netip.MustParseAddr("2001:db8::1"))
	if !c.Documentation {
		t.Errorf("2001:db8::1 should be Documentation")
	}
}

func TestClassify_IPv6UniqueLocal(t *testing.T) {
	c := Classify(netip.MustParseAddr("fd00::1"))
	if !c.UniqueLocal {
		t.Errorf("fd00::1 should be UniqueLocal")
	}
	// ULA 也算 Private（RFC 4193）
	if !c.Private {
		t.Errorf("ULA should also be Private")
	}
}

// ---- ReverseDNS ----

func TestReverseDNS_IPv4(t *testing.T) {
	got := ReverseDNS(netip.MustParseAddr("192.168.1.42"))
	want := "42.1.168.192.in-addr.arpa"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReverseDNS_IPv4Zero(t *testing.T) {
	got := ReverseDNS(netip.MustParseAddr("0.0.0.0"))
	want := "0.0.0.0.in-addr.arpa"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReverseDNS_IPv6Basic(t *testing.T) {
	got := ReverseDNS(netip.MustParseAddr("2001:db8::1"))
	// 32 nibbles + ".ip6.arpa"
	if !strings.HasSuffix(got, ".ip6.arpa") {
		t.Errorf("missing ip6.arpa suffix: %q", got)
	}
	// 末尾应该是 "...8.b.d.0.1.0.0.2.ip6.arpa"（2001:0db8 倒序的高位）
	if !strings.HasSuffix(got, "8.b.d.0.1.0.0.2.ip6.arpa") {
		t.Errorf("IPv6 reverse DNS wrong suffix: %q", got)
	}
	// nibble count: 32 个 nibble + 32 个 dot + "ip6.arpa" (8 chars)
	// 实际看：每 nibble 形如 "X."，共 32*2=64 chars + "ip6.arpa" 8 chars = 72
	if len(got) != 72 {
		t.Errorf("expected 72 chars, got %d: %q", len(got), got)
	}
}

// ---- CIDR ----

func TestComputeCIDR_IPv4Slash24(t *testing.T) {
	p := netip.MustParsePrefix("192.168.1.0/24")
	info, err := ComputeCIDR(p)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"network":   "192.168.1.0",
		"broadcast": "192.168.1.255",
		"first":     "192.168.1.1",
		"last":      "192.168.1.254",
		"netmask":   "255.255.255.0",
		"wildcard":  "0.0.0.255",
	}
	if info.Network.String() != checks["network"] {
		t.Errorf("network = %s", info.Network)
	}
	if info.Broadcast.String() != checks["broadcast"] {
		t.Errorf("broadcast = %s", info.Broadcast)
	}
	if info.FirstHost.String() != checks["first"] {
		t.Errorf("first = %s", info.FirstHost)
	}
	if info.LastHost.String() != checks["last"] {
		t.Errorf("last = %s", info.LastHost)
	}
	if info.Netmask.String() != checks["netmask"] {
		t.Errorf("netmask = %s", info.Netmask)
	}
	if info.Wildcard.String() != checks["wildcard"] {
		t.Errorf("wildcard = %s", info.Wildcard)
	}
	if info.TotalAddrs.String() != "256" {
		t.Errorf("total = %s", info.TotalAddrs)
	}
	if info.UsableAddrs.String() != "254" {
		t.Errorf("usable = %s", info.UsableAddrs)
	}
}

func TestComputeCIDR_IPv4Slash31(t *testing.T) {
	// /31 是 RFC 3021 point-to-point：2 个地址都算 usable
	p := netip.MustParsePrefix("192.168.1.0/31")
	info, _ := ComputeCIDR(p)
	if info.TotalAddrs.String() != "2" {
		t.Errorf("total = %s", info.TotalAddrs)
	}
	if info.UsableAddrs.String() != "2" {
		t.Errorf("usable should be 2 for /31 (RFC 3021), got %s", info.UsableAddrs)
	}
}

func TestComputeCIDR_IPv4Slash32(t *testing.T) {
	// /32 = 单 IP，usable = 1
	p := netip.MustParsePrefix("192.168.1.42/32")
	info, _ := ComputeCIDR(p)
	if info.TotalAddrs.String() != "1" {
		t.Errorf("total = %s", info.TotalAddrs)
	}
	if info.UsableAddrs.String() != "1" {
		t.Errorf("usable should be 1 for /32, got %s", info.UsableAddrs)
	}
}

func TestComputeCIDR_IPv4Slash0(t *testing.T) {
	p := netip.MustParsePrefix("0.0.0.0/0")
	info, _ := ComputeCIDR(p)
	if info.Network.String() != "0.0.0.0" {
		t.Errorf("network = %s", info.Network)
	}
	if info.Broadcast.String() != "255.255.255.255" {
		t.Errorf("broadcast = %s", info.Broadcast)
	}
	if info.TotalAddrs.String() != "4294967296" {
		t.Errorf("total = %s", info.TotalAddrs)
	}
	if info.Netmask.String() != "0.0.0.0" {
		t.Errorf("netmask = %s", info.Netmask)
	}
}

func TestComputeCIDR_IPv6Slash64(t *testing.T) {
	p := netip.MustParsePrefix("2001:db8::/64")
	info, _ := ComputeCIDR(p)
	if info.Network.String() != "2001:db8::" {
		t.Errorf("network = %s", info.Network)
	}
	// /64 last = 2001:db8:0:0:ffff:ffff:ffff:ffff
	if !strings.HasSuffix(info.Broadcast.String(), "ffff:ffff:ffff:ffff") {
		t.Errorf("broadcast = %s", info.Broadcast)
	}
	if info.TotalAddrs.String() != "18446744073709551616" {
		t.Errorf("total = %s (want 2^64)", info.TotalAddrs)
	}
	// IPv6: usable == total
	if info.UsableAddrs.Cmp(info.TotalAddrs) != 0 {
		t.Errorf("IPv6 usable should == total")
	}
}

func TestComputeCIDR_AutoMaskedAlignment(t *testing.T) {
	// 192.168.1.42/24 → network 是 192.168.1.0（不是 .42）
	p := netip.MustParsePrefix("192.168.1.42/24")
	info, _ := ComputeCIDR(p)
	if info.Network.String() != "192.168.1.0" {
		t.Errorf("ComputeCIDR should auto-Mask; got %s", info.Network)
	}
}

// ---- Range ----

func TestRange_Limit(t *testing.T) {
	p := netip.MustParsePrefix("192.168.1.0/29")
	res, err := Range(p, 16)
	if err != nil {
		t.Fatal(err)
	}
	// /29 = 8 个地址，limit=16 → 返回 8 个
	if len(res.Addrs) != 8 {
		t.Errorf("len = %d, want 8", len(res.Addrs))
	}
	if res.Returned != 8 {
		t.Errorf("Returned = %d", res.Returned)
	}
	if res.Addrs[0].String() != "192.168.1.0" {
		t.Errorf("first = %s", res.Addrs[0])
	}
	if res.Addrs[7].String() != "192.168.1.7" {
		t.Errorf("last = %s", res.Addrs[7])
	}
}

func TestRange_LimitTruncates(t *testing.T) {
	p := netip.MustParsePrefix("192.168.1.0/24")
	res, _ := Range(p, 3)
	if len(res.Addrs) != 3 {
		t.Errorf("expected 3, got %d", len(res.Addrs))
	}
}

func TestRange_LargeCIDR_NoLimit_Errors(t *testing.T) {
	// /8 = 16M > hardCap，limit=0 必须报错
	p := netip.MustParsePrefix("10.0.0.0/8")
	_, err := Range(p, 0)
	if err == nil {
		t.Error("expected error for too-large CIDR with limit=0")
	}
}

func TestRange_LargeCIDR_WithLimit_OK(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/8")
	res, err := Range(p, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Addrs) != 5 {
		t.Errorf("got %d, want 5", len(res.Addrs))
	}
}

// ---- Split ----

func TestSplit_Basic(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/22")
	subnets, err := Split(p, 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(subnets) != 4 {
		t.Fatalf("got %d, want 4", len(subnets))
	}
	want := []string{
		"10.0.0.0/24",
		"10.0.1.0/24",
		"10.0.2.0/24",
		"10.0.3.0/24",
	}
	for i, w := range want {
		if subnets[i].String() != w {
			t.Errorf("subnets[%d] = %s, want %s", i, subnets[i], w)
		}
	}
}

func TestSplit_SamePrefix(t *testing.T) {
	// newBits == parent.Bits() → 1 个子网就是 parent 自己
	p := netip.MustParsePrefix("10.0.0.0/24")
	subnets, _ := Split(p, 24)
	if len(subnets) != 1 {
		t.Errorf("got %d, want 1", len(subnets))
	}
}

func TestSplit_NewBitsTooSmall_Errors(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/22")
	_, err := Split(p, 20) // 20 < 22
	if err == nil {
		t.Error("expected error when newBits < parent bits")
	}
}

func TestSplit_TooManySubnets_Errors(t *testing.T) {
	// /8 split /24 = 65536 subnets ＞ maxSplitCount(65536) → 不报错（边界）
	// /8 split /25 = 131072 subnets ＞ maxSplitCount → 报错
	p := netip.MustParsePrefix("10.0.0.0/8")
	_, err := Split(p, 25)
	if err == nil {
		t.Error("expected error for too many subnets")
	}
}

func TestSplit_IPv6(t *testing.T) {
	p := netip.MustParsePrefix("2001:db8::/62")
	subnets, err := Split(p, 64)
	if err != nil {
		t.Fatal(err)
	}
	if len(subnets) != 4 {
		t.Errorf("got %d, want 4", len(subnets))
	}
}

// ---- Normalize ----

func TestNormalize_IPv6Compact(t *testing.T) {
	addr := netip.MustParseAddr("2001:0db8:0000:0000:0000:0000:0000:0001")
	got, _ := Normalize(addr, false)
	if got != "2001:db8::1" {
		t.Errorf("got %q, want 2001:db8::1", got)
	}
}

func TestNormalize_IPv6Expand(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	got, _ := Normalize(addr, true)
	if got != "2001:0db8:0000:0000:0000:0000:0000:0001" {
		t.Errorf("got %q", got)
	}
}

func TestNormalize_IPv4Unchanged(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.1")
	got, _ := Normalize(addr, true) // expand on IPv4 no-op
	if got != "192.168.1.1" {
		t.Errorf("got %q", got)
	}
}

// ---- AddrInfo ----

func TestComputeAddrInfo_IPv4(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.42")
	info := ComputeAddrInfo(addr)
	if info.Version != 4 {
		t.Errorf("Version = %d", info.Version)
	}
	if info.Hex != "0xC0A8012A" {
		t.Errorf("Hex = %s", info.Hex)
	}
	if info.Decimal != "3232235818" {
		t.Errorf("Decimal = %s", info.Decimal)
	}
	if !info.Classification.Private {
		t.Errorf("should classify as Private")
	}
}

func TestComputeAddrInfo_IPv6(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	info := ComputeAddrInfo(addr)
	if info.Version != 6 {
		t.Errorf("Version = %d", info.Version)
	}
	if info.Compact != "2001:db8::1" {
		t.Errorf("Compact = %s", info.Compact)
	}
	if info.Expanded != "2001:0db8:0000:0000:0000:0000:0000:0001" {
		t.Errorf("Expanded = %s", info.Expanded)
	}
	if !info.Classification.Documentation {
		t.Errorf("should classify as Documentation")
	}
}
