package barcode

import "testing"

func TestEncode_ChecksumHandComputed(t *testing.T) {
	// 校验 = (Start + Σ 位置×值) mod 103，手算钉死。
	// "A"（B集）：startB=104, 'A'=65-32=33 → (104+1*33)=137 mod103=34
	if s, err := Encode("A"); err != nil || s.Checksum != 34 || s.CodeSet != "B" {
		t.Errorf(`Encode("A") checksum=%d set=%s err=%v，期望 34/B`, s.Checksum, s.CodeSet, err)
	}
	// "AB"：'A'=33,'B'=34 → (104+33+2*34)=205 mod103=102
	if s, _ := Encode("AB"); s.Checksum != 102 {
		t.Errorf(`Encode("AB") checksum=%d，期望 102`, s.Checksum)
	}
	// "00"（全数字偶数 → C集）：startC=105, 值=0 → 105 mod103=2
	if s, _ := Encode("00"); s.Checksum != 2 || s.CodeSet != "C" {
		t.Errorf(`Encode("00") checksum=%d set=%s，期望 2/C`, s.Checksum, s.CodeSet)
	}
}

func TestEncode_CodeSetSelection(t *testing.T) {
	if s, _ := Encode("12345678"); s.CodeSet != "C" {
		t.Errorf("全数字偶数应 C集，got %s", s.CodeSet)
	}
	if s, _ := Encode("1234567"); s.CodeSet != "B" { // 奇数位回退 B
		t.Errorf("奇数位数字应回退 B集，got %s", s.CodeSet)
	}
	if s, _ := Encode("ABC123"); s.CodeSet != "B" {
		t.Errorf("含字母应 B集，got %s", s.CodeSet)
	}
}

// C 集应比 B 集窄：同样 8 位数字，C 编 4 个符号、B 编 8 个符号。
func TestEncode_CodeSetCIsNarrower(t *testing.T) {
	c, _ := Encode("12345678") // C集
	b, _ := Encode("1234567a") // 含字母强制 B集（同 8 字符）
	if c.Width() >= b.Width() {
		t.Errorf("C集应更窄：C=%d B=%d", c.Width(), b.Width())
	}
}

func TestEncode_ModuleCount(t *testing.T) {
	// "A"：Start(11)+数据(11)+校验(11)+Stop(13)=46，+两侧静区 20 = 66
	s, _ := Encode("A")
	if s.Width() != 66 {
		t.Errorf(`Encode("A").Width()=%d，期望 66`, s.Width())
	}
}

func TestEncode_QuietZones(t *testing.T) {
	s, _ := Encode("A")
	for i := range quietModules {
		if s.Modules[i] {
			t.Fatalf("前 %d 模块应为静区（白），第 %d 个是黑", quietModules, i)
		}
		if s.Modules[len(s.Modules)-1-i] {
			t.Fatalf("末 %d 模块应为静区（白）", quietModules)
		}
	}
	// 静区后第一个模块是 Start 的首条，应为黑
	if !s.Modules[quietModules] {
		t.Error("静区后应紧跟 Start 首条（黑）")
	}
}

func TestEncode_Errors(t *testing.T) {
	if _, err := Encode(""); err == nil {
		t.Error("空输入应报错")
	}
	if _, err := Encode("中文"); err == nil {
		t.Error("非 ASCII 应报错")
	}
	if _, err := Encode("a\tb"); err == nil {
		t.Error("控制字符（tab）应报错")
	}
}

// 模式表完整性：107 行，0-105 每行 6 元素和为 11，Stop(106) 和为 13。
func TestPatternTableIntegrity(t *testing.T) {
	for v, p := range code128Patterns {
		sum := 0
		for _, w := range p {
			sum += int(w - '0')
		}
		want := 11
		if v == 106 {
			want = 13
		}
		if sum != want {
			t.Errorf("模式 %d=%q 模块和=%d，期望 %d", v, p, sum, want)
		}
	}
}
