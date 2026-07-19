// Package sizex 统计目录树的磁盘占用。
//
// 默认量的是**实际占盘**（st_blocks × 512）而不是逻辑大小（Size()）。两者
// 在主要使用场景下差很多，且方向常被搞反：
//
//	500 个 1 字节文件   逻辑 500 B      实际 2.0 MB   （4 KiB 块取整，4000×）
//	/etc/hosts          逻辑 366 B      实际 4096 B   （11×）
//	1 GiB 稀疏文件      逻辑 1 GiB      实际 0 B      （反向）
//
// 对 node_modules / .git / Caches 这类大量小文件的目录，块取整是主导项，
// 实际占盘远大于逻辑大小。用户问的是「删掉能腾出多少空间」，所以默认必须
// 是实际占盘。
//
// 语义对齐 du：硬链接只计一次、默认不跨文件系统、不跟随符号链接、目录
// 自身的 st_blocks 计入总量。
package sizex

import "os"

// fileStat 是从 syscall.Stat_t 抽出来的、跨平台统一的那几个字段。
type fileStat struct {
	bytes uint64 // 实际占盘（st_blocks × 512）；不支持的平台退回 Size()
	dev   uint64 // 设备号，用于跨文件系统检测
	ino   uint64 // inode 号，与 dev 一起做硬链接去重的 key
	nlink uint64 // 硬链接数；> 1 才需要进延迟队列
}

// Stat 返回路径的占盘信息。ok=false 表示本平台拿不到 st_blocks，
// bytes 退化为逻辑大小，dev/ino/nlink 无意义。
//
// 传入的 info 必须来自 os.Lstat（不跟随符号链接），否则符号链接会被当成
// 它指向的文件重复计数，且可能绕出环。
func Stat(info os.FileInfo) (bytes, dev, ino, nlink uint64, ok bool) {
	fs, ok := statOf(info)
	return fs.bytes, fs.dev, fs.ino, fs.nlink, ok
}
