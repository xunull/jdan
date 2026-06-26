//go:build darwin

package diskx

import "syscall"

// MNT_NOWAIT：不等待刷新，直接返回缓存的挂载信息。stdlib syscall(darwin) 未导出，写字面量。
const mntNoWait = 0x2

// Mounts 用 syscall.Getfsstat 一次拿全部挂载点（含设备名/挂载点/类型/块数）。
func Mounts() ([]Mount, error) {
	n, err := syscall.Getfsstat(nil, mntNoWait)
	if err != nil {
		return nil, err
	}
	buf := make([]syscall.Statfs_t, n)
	n, err = syscall.Getfsstat(buf, mntNoWait)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]
	out := make([]Mount, 0, len(buf))
	for i := range buf {
		out = append(out, statfsToMount(&buf[i]))
	}
	return out, nil
}

// StatPath 返回某路径所在文件系统的容量信息。
func StatPath(path string) (Mount, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Mount{}, err
	}
	return statfsToMount(&st), nil
}

func statfsToMount(st *syscall.Statfs_t) Mount {
	return Mount{
		Device:     int8ToString(st.Mntfromname[:]),
		Mountpoint: int8ToString(st.Mntonname[:]),
		Fstype:     int8ToString(st.Fstypename[:]),
		BlockSize:  uint64(st.Bsize),
		Blocks:     st.Blocks,
		Bfree:      st.Bfree,
		Bavail:     st.Bavail,
		Files:      st.Files,
		Ffree:      st.Ffree,
	}
}

// darwin 的 Statfs_t 字符数组是 [N]int8，转成 string（截到 NUL）。
func int8ToString(b []int8) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	out := make([]byte, n)
	for i := range n {
		out[i] = byte(b[i])
	}
	return string(out)
}
