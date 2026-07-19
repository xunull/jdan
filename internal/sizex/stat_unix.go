//go:build darwin || linux

package sizex

import (
	"os"
	"syscall"
)

// Supported 表示本平台能拿到真实占盘和 inode 信息。
const Supported = true

// statOf 从 os.FileInfo 抽出占盘字节、设备号、inode 号和硬链接数。
//
// 为什么 darwin 和 linux 共用一个文件：Stat_t 的字段类型在两个平台（乃至
// 不同 GOARCH）上并不一致，但每个字段这里都做了显式转换，所以函数体本身
// 逐字节相同，没有理由拆成两个文件。diskx 拆成 diskx_darwin.go /
// diskx_linux.go 是因为它们的函数体真不同（Getfsstat vs /proc/self/mounts），
// 别为了形似而照抄那个结构。
//
// 各字段的平台差异：
//
//	          darwin    linux/386,arm    linux/amd64,arm64
//	Dev       int32     uint64           uint64
//	Ino       uint64    uint64           uint64
//	Nlink     uint16    uint32           uint64
//	Blocks    int64     int64            int64
//
// 三向分歧的是 Nlink，不是 Dev —— 单测按这个来。
func statOf(info os.FileInfo) (fileStat, bool) {
	s, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileStat{bytes: uint64(info.Size())}, false
	}
	return fileStat{
		// st_blocks 恒以 512 字节为单位，与文件系统块大小无关：APFS 块是
		// 4096 时 /etc/hosts（366 B）的 Blocks 仍是 8 → 4096 B。
		bytes: uint64(s.Blocks) * 512,
		// darwin 上 Dev 是 int32，为负时 uint64() 会符号扩展成
		// 0xFFFFFFFFxxxxxxxx。这无害：Dev 只做相等比较（去重、跨盘判断），
		// 符号扩展是单射的。不要套 uint32() —— 那会把 linux 的 uint64 dev_t
		// 静默窄化，major=4096,minor=3 (0x100000000003) 会与
		// major=0,minor=3 (0x3) 碰撞，导致跨盘误判和错误去重。
		dev:   uint64(s.Dev),
		ino:   uint64(s.Ino),
		nlink: uint64(s.Nlink),
	}, true
}
