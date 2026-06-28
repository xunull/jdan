package pwned

import "testing"

func TestSHA1Hex(t *testing.T) {
	// 已知常量：SHA1("password") 大写十六进制
	if got := SHA1Hex("password"); got != "5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8" {
		t.Errorf("SHA1Hex(password)=%s", got)
	}
}

func TestSplitRange(t *testing.T) {
	p, s := SplitRange(SHA1Hex("password"))
	if p != "5BAA6" {
		t.Errorf("prefix=%q want 5BAA6", p)
	}
	if s != "1E4C9B93F3F0682250B6CF8331B7EE68FD8" {
		t.Errorf("suffix=%q", s)
	}
	if len(p) != 5 || len(s) != 35 {
		t.Errorf("k-匿名拆分长度应为 5+35，got %d+%d", len(p), len(s))
	}
}

func TestLookup(t *testing.T) {
	body := "0000000000000000000000000000000000A:0\r\n" +
		"1E4C9B93F3F0682250B6CF8331B7EE68FD8:99\r\n" +
		"BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB:5"

	if n := Lookup(body, "1E4C9B93F3F0682250B6CF8331B7EE68FD8"); n != 99 {
		t.Errorf("命中应返回 99，got %d", n)
	}
	if n := Lookup(body, "1e4c9b93f3f0682250b6cf8331b7ee68fd8"); n != 99 {
		t.Errorf("小写 suffix 也应命中 99，got %d", n)
	}
	if n := Lookup(body, "DEADBEEF00000000000000000000000000A"); n != 0 {
		t.Errorf("未命中应为 0，got %d", n)
	}
	// Add-Padding 塞入的 count=0 假条目应被当作未泄露
	if n := Lookup(body, "0000000000000000000000000000000000A"); n != 0 {
		t.Errorf("padding :0 应视为 0，got %d", n)
	}
}
