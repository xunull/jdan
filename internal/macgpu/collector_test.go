package macgpu

import (
	"bytes"
	"testing"
)

func TestCollectOnce_TrimsNUL(t *testing.T) {
	// powermetrics plist 输出以 \x00 结尾，验证 TrimRight 能正确去除后交给解析器。
	plistWithNUL := []byte(m1PlistFixture + "\x00")
	data := bytes.TrimRight(plistWithNUL, "\x00")
	data = bytes.TrimSpace(data)

	if len(data) == 0 {
		t.Fatal("期望 TrimRight 后仍有内容")
	}

	snapshot, err := ParseSample(data)
	if err != nil {
		t.Fatalf("意外解析错误: %v", err)
	}
	if snapshot.ActiveResidency < 0 || snapshot.ActiveResidency > 1 {
		t.Errorf("ActiveResidency = %v 超出范围 [0, 1]", snapshot.ActiveResidency)
	}
}

func TestCollectOnce_EmptyOutputSkipped(t *testing.T) {
	// 空输出（或全为 NUL）不应触发解析错误，直接跳过。
	cases := [][]byte{
		{},
		{0x00},
		{0x00, 0x00},
	}
	for _, input := range cases {
		data := bytes.TrimRight(input, "\x00")
		data = bytes.TrimSpace(data)
		if len(data) != 0 {
			t.Errorf("期望处理后为空，但得到 %q", data)
		}
	}
}
