package unixtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const outputLayout = "2006-01-02 15:04:05 -07:00"

var (
	ErrEmptyInput    = errors.New("时间戳不能为空")
	ErrInvalidNumber = errors.New("时间戳必须为纯数字")
	ErrInvalidLength = errors.New("仅支持 10 位（秒）或 13 位（毫秒）时间戳")
)

// Convert 将 Unix 时间戳（秒或毫秒）转换为本地时区可读时间字符串。
// 规则：
// - 10 位按秒处理
// - 13 位按毫秒处理
func Convert(raw string) (string, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return "", ErrEmptyInput
	}

	for _, r := range input {
		if r < '0' || r > '9' {
			return "", ErrInvalidNumber
		}
	}

	switch len(input) {
	case 10:
		sec, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return "", fmt.Errorf("秒级时间戳解析失败: %w", err)
		}
		return time.Unix(sec, 0).In(time.Local).Format(outputLayout), nil
	case 13:
		ms, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return "", fmt.Errorf("毫秒级时间戳解析失败: %w", err)
		}
		sec := ms / 1000
		nsec := (ms % 1000) * int64(time.Millisecond)
		return time.Unix(sec, nsec).In(time.Local).Format(outputLayout), nil
	default:
		return "", ErrInvalidLength
	}
}
