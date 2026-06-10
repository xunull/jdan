package randgen

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

// GenerateUUIDv4 返回符合 RFC 9562 的 v4 UUID（122 个随机比特 + 4-bit 版本号 4 +
// 2-bit variant 10）。
//
// 字节布局：
//
//	[0-5]   完全随机
//	[6]     高 4 bits = 0x4（版本），低 4 bits = 随机
//	[7]     完全随机
//	[8]     高 2 bits = 0b10（variant），低 6 bits = 随机
//	[9-15]  完全随机
func GenerateUUIDv4(reader io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(reader, b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version = 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant = 10
	return formatUUID(b), nil
}

// GenerateUUIDv7 返回符合 RFC 9562 的 v7 UUID（时间排序 + 随机熵）。
//
// 字节布局：
//
//	[0-5]   48-bit unix 毫秒时间戳（big-endian）
//	[6]     高 4 bits = 0x7（版本），低 4 bits = rand_a (12 bits 共)
//	[7]     rand_a 低 8 bits
//	[8]     高 2 bits = 0b10（variant），低 6 bits = rand_b 高 6 bits
//	[9-15]  rand_b 低 56 bits
//
// 同毫秒内多次生成会通过 rand_a 提供"大致单调"排序（不严格 sub-ms 单调）。
// 拿了系统时间戳意味着 `time.Now()` 必须正确——系统时钟错乱会让 v7 失去排序意义。
func GenerateUUIDv7(reader io.Reader) (string, error) {
	ms := time.Now().UnixMilli()
	if ms < 0 {
		return "", errors.New("system clock before 1970, v7 timestamp invalid")
	}

	var b [16]byte
	// 48-bit 时间戳 big-endian 写入 byte 0..5
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)

	// 余下 10 字节随机填充
	if _, err := io.ReadFull(reader, b[6:]); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x70 // version = 7
	b[8] = (b[8] & 0x3f) | 0x80 // variant = 10

	return formatUUID(b), nil
}

// formatUUID 把 16 字节渲染为 canonical 8-4-4-4-12 hex 形式。
func formatUUID(b [16]byte) string {
	s := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		s[0:8], s[8:12], s[12:16], s[16:20], s[20:32])
}
