package httpserve

import (
	"net"
	"testing"
)

func TestIsRFC1918(t *testing.T) {
	for _, tc := range []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"192.168.255.255", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.15.0.1", false}, // 边界外
		{"172.32.0.1", false}, // 边界外
		{"8.8.8.8", false},    // 公网
		{"1.1.1.1", false},
		{"127.0.0.1", false},  // loopback
		{"169.254.1.1", false}, // link-local
		{"0.0.0.0", false},
		{"::1", false}, // IPv6
	} {
		ip := net.ParseIP(tc.ip)
		if got := isRFC1918(ip); got != tc.want {
			t.Errorf("isRFC1918(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

func TestSortLANIPs_192First(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("10.0.0.5"),
		net.ParseIP("192.168.1.42"),
		net.ParseIP("172.20.0.1"),
	}
	sortLANIPs(ips)
	if ips[0].String() != "192.168.1.42" {
		t.Errorf("192.168 should be first, got %s", ips[0])
	}
	if ips[1].String() != "10.0.0.5" {
		t.Errorf("10.x should be second, got %s", ips[1])
	}
	if ips[2].String() != "172.20.0.1" {
		t.Errorf("172.x should be third, got %s", ips[2])
	}
}

func TestSortLANIPs_LexWithinGroup(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("192.168.5.1"),
		net.ParseIP("192.168.1.1"),
		net.ParseIP("192.168.3.1"),
	}
	sortLANIPs(ips)
	want := []string{"192.168.1.1", "192.168.3.1", "192.168.5.1"}
	for i, w := range want {
		if ips[i].String() != w {
			t.Errorf("[%d] got %s, want %s", i, ips[i], w)
		}
	}
}

func TestExtractIPv4_FiltersLinkLocal(t *testing.T) {
	addr := &net.IPNet{
		IP:   net.ParseIP("169.254.1.1"),
		Mask: net.CIDRMask(16, 32),
	}
	if extractIPv4(addr) != nil {
		t.Error("link-local 169.254.x.x should be filtered out by extractIPv4")
	}
}

func TestExtractIPv4_FiltersLoopback(t *testing.T) {
	addr := &net.IPNet{
		IP:   net.ParseIP("127.0.0.1"),
		Mask: net.CIDRMask(8, 32),
	}
	if extractIPv4(addr) != nil {
		t.Error("loopback should be filtered out by extractIPv4")
	}
}

func TestExtractIPv4_FiltersIPv6(t *testing.T) {
	addr := &net.IPNet{
		IP:   net.ParseIP("fe80::1"),
		Mask: net.CIDRMask(64, 128),
	}
	if extractIPv4(addr) != nil {
		t.Error("IPv6 link-local should be filtered out (no IPv4 form)")
	}
}

func TestExtractIPv4_AcceptsPrivate(t *testing.T) {
	addr := &net.IPNet{
		IP:   net.ParseIP("192.168.1.42"),
		Mask: net.CIDRMask(24, 32),
	}
	got := extractIPv4(addr)
	if got == nil {
		t.Fatal("private IPv4 should pass extractIPv4")
	}
	if got.String() != "192.168.1.42" {
		t.Errorf("got %s, want 192.168.1.42", got)
	}
}

// 真实接口枚举：只断言不报错 + 结果都是 RFC1918（在 CI 上 LAN IP 可能为空）
func TestDetectLANIPs_RealInterfaces_AllPrivate(t *testing.T) {
	ips, err := DetectLANIPs()
	if err != nil {
		t.Fatalf("real interface enumeration failed: %v", err)
	}
	for _, ip := range ips {
		if !isRFC1918(ip) {
			t.Errorf("DetectLANIPs returned non-RFC1918 address: %s", ip)
		}
		if ip.IsLoopback() {
			t.Errorf("loopback leaked: %s", ip)
		}
	}
}
