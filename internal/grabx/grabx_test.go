package grabx

import (
	"slices"
	"testing"
)

func TestURLs(t *testing.T) {
	got := URLs(`see https://a.com/x?q=1 and http://b.org. also ftp://c.net) plus not-a-url`)
	want := []string{"https://a.com/x?q=1", "http://b.org", "ftp://c.net"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestEmails(t *testing.T) {
	got := Emails(`reach alice@example.com or bob@sub.example.co.uk; junk a@@b and x@y (no tld)`)
	want := []string{"alice@example.com", "bob@sub.example.co.uk"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestIPs(t *testing.T) {
	got := IPs(`v4 10.0.0.1 8.8.8.8 bad 999.1.1.1 256.0.0.1 | v6 2001:db8::1 | mac aa:bb:cc:dd:ee:ff | time 12:34:56`)
	// 真 IP 留下；999/256 越界、MAC、time 被 netip 淘汰
	for _, want := range []string{"10.0.0.1", "8.8.8.8", "2001:db8::1"} {
		if !slices.Contains(got, want) {
			t.Errorf("应抽到 %q，got %v", want, got)
		}
	}
	for _, bad := range []string{"999.1.1.1", "256.0.0.1", "aa:bb:cc:dd:ee:ff", "12:34:56"} {
		if slices.Contains(got, bad) {
			t.Errorf("不该抽到 %q（应被 stdlib 校验淘汰），got %v", bad, got)
		}
	}
}

func TestDedup(t *testing.T) {
	got := Dedup([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"} // 去重 + 首次出现顺序
	if !slices.Equal(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestIPs_NormalizesV6(t *testing.T) {
	// netip 归一化：压缩零段
	got := IPs("addr 2001:0db8:0000:0000:0000:0000:0000:0001 end")
	if len(got) != 1 || got[0] != "2001:db8::1" {
		t.Errorf("v6 应归一化为 2001:db8::1，got %v", got)
	}
}
