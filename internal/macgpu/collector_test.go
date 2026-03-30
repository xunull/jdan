package macgpu

import (
	"bytes"
	"testing"
)

func TestSplitOnNUL(t *testing.T) {
	tests := []struct {
		name         string
		data         []byte
		atEOF        bool
		wantAdvance  int
		wantToken    []byte
		wantNilToken bool
	}{
		{
			name:        "包含 NUL 分隔符",
			data:        []byte("hello\x00world"),
			atEOF:       false,
			wantAdvance: 6,
			wantToken:   []byte("hello"),
		},
		{
			name:        "NUL 在开头",
			data:        []byte("\x00rest"),
			atEOF:       false,
			wantAdvance: 1,
			wantToken:   []byte(""),
		},
		{
			name:         "无 NUL 且非 EOF",
			data:         []byte("partial data"),
			atEOF:        false,
			wantAdvance:  0,
			wantNilToken: true,
		},
		{
			name:        "无 NUL 但 atEOF=true（最后一块）",
			data:        []byte("last chunk"),
			atEOF:       true,
			wantAdvance: 10,
			wantToken:   []byte("last chunk"),
		},
		{
			name:         "空数据 atEOF=true",
			data:         []byte{},
			atEOF:        true,
			wantAdvance:  0,
			wantNilToken: true,
		},
		{
			name:         "空数据 atEOF=false",
			data:         []byte{},
			atEOF:        false,
			wantAdvance:  0,
			wantNilToken: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			advance, token, err := splitOnNUL(tc.data, tc.atEOF)
			if err != nil {
				t.Fatalf("意外错误: %v", err)
			}
			if advance != tc.wantAdvance {
				t.Errorf("advance = %d，期望 %d", advance, tc.wantAdvance)
			}
			if tc.wantNilToken {
				if token != nil {
					t.Errorf("期望 token == nil，但得到 %q", token)
				}
			} else {
				if !bytes.Equal(token, tc.wantToken) {
					t.Errorf("token = %q，期望 %q", token, tc.wantToken)
				}
			}
		})
	}
}

func TestSplitOnNUL_MultipleSeparators(t *testing.T) {
	data := []byte("block1\x00block2\x00block3")
	var tokens []string

	// 模拟 bufio.Scanner 的分割行为
	for len(data) > 0 {
		advance, token, err := splitOnNUL(data, false)
		if err != nil {
			t.Fatal(err)
		}
		if advance == 0 {
			// 没有更多分隔符时，作为最后一块处理
			_, token, _ = splitOnNUL(data, true)
			tokens = append(tokens, string(token))
			break
		}
		tokens = append(tokens, string(token))
		data = data[advance:]
	}

	want := []string{"block1", "block2", "block3"}
	if len(tokens) != len(want) {
		t.Fatalf("token 数量 = %d，期望 %d；tokens = %v", len(tokens), len(want), tokens)
	}
	for i := range want {
		if tokens[i] != want[i] {
			t.Errorf("tokens[%d] = %q，期望 %q", i, tokens[i], want[i])
		}
	}
}
