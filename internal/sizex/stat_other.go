//go:build !darwin && !linux

package sizex

import "os"

// Supported 为 false：本平台拿不到 st_blocks / inode，只能退回逻辑大小。
// 上层据此关闭硬链接去重与跨文件系统检测，并在输出头部提示用户。
//
// 注意这里是**降级**而不是报错。diskx_other.go 返回 errUnsupported 是因为
// 「列挂载点」在没有 syscall 时根本做不了；而「统计目录大小」用 Size() 仍
// 然有用，只是数字含义不同（逻辑大小而非占盘）。
const Supported = false

func statOf(info os.FileInfo) (fileStat, bool) {
	return fileStat{bytes: uint64(info.Size()), nlink: 1}, false
}
