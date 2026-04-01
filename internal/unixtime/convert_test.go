package unixtime

import (
	"errors"
	"testing"
	"time"
)

func TestConvert(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC+8", 8*3600)
	defer func() {
		time.Local = originalLocal
	}()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{
			name:  "10位秒级时间戳",
			input: "1711843200",
			want:  "2024-03-31 08:00:00 +08:00",
		},
		{
			name:  "13位毫秒级时间戳",
			input: "1711843200123",
			want:  "2024-03-31 08:00:00 +08:00",
		},
		{
			name:  "前后空白会被trim",
			input: " 1711843200 \n",
			want:  "2024-03-31 08:00:00 +08:00",
		},
		{
			name:    "非数字输入报错",
			input:   "abc",
			wantErr: ErrInvalidNumber,
		},
		{
			name:    "位数非法报错",
			input:   "17118432",
			wantErr: ErrInvalidLength,
		},
		{
			name:    "空输入报错",
			input:   " ",
			wantErr: ErrEmptyInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Convert(tt.input)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
