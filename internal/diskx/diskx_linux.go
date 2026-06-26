//go:build linux

package diskx

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Mounts 解析 /proc/self/mounts 枚举挂载点，再对每个 statfs 拿块数。
func Mounts() ([]Mount, error) {
	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Mount
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		mountpoint := unescapeMount(fields[1])
		var st syscall.Statfs_t
		if err := syscall.Statfs(mountpoint, &st); err != nil {
			continue // 无权限 / 瞬时卸载，跳过
		}
		out = append(out, Mount{
			Device:     unescapeMount(fields[0]),
			Mountpoint: mountpoint,
			Fstype:     fields[2],
			BlockSize:  blockSize(&st),
			Blocks:     st.Blocks,
			Bfree:      st.Bfree,
			Bavail:     st.Bavail,
			Files:      st.Files,
			Ffree:      st.Ffree,
		})
	}
	return out, sc.Err()
}

// StatPath 返回某路径所在文件系统的容量信息。
func StatPath(path string) (Mount, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return Mount{}, err
	}
	return Mount{
		Mountpoint: path,
		BlockSize:  blockSize(&st),
		Blocks:     st.Blocks,
		Bfree:      st.Bfree,
		Bavail:     st.Bavail,
		Files:      st.Files,
		Ffree:      st.Ffree,
	}, nil
}

func blockSize(st *syscall.Statfs_t) uint64 {
	if st.Frsize > 0 {
		return uint64(st.Frsize)
	}
	return uint64(st.Bsize)
}

// /proc/mounts 把空格转义成 \040、tab \011 等八进制，还原它。
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+4 <= len(s) {
			if v, err := strconv.ParseInt(s[i+1:i+4], 8, 16); err == nil {
				b.WriteByte(byte(v))
				i += 4
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}
