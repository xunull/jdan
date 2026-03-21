package filebak

import (
	"fmt"
	"testing"
)

func TestNormalizeDesc(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"", "", true},
		{"  ", "", true},
		{"a b", "a_b", true},
		{"ab", "ab", true},
		{"中文", "中文", true},
		{"a,b", "", false},
		{"a\tb", "", false},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			got, ok := NormalizeDesc(tt.in)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantOK && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
