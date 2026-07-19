package sizex

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func lstat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	return info
}

// 核心断言：实际占盘按 512 字节块计，与文件系统块大小无关。
func TestStat_BlocksAreMultiplesOf512(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	bytes, _, _, _, ok := Stat(lstat(t, p))
	if !ok {
		t.Fatal("Supported=true 但 Stat 返回 ok=false")
	}
	if bytes%512 != 0 {
		t.Errorf("占盘 %d 不是 512 的整数倍", bytes)
	}
}

// 块取整：小文件的实际占盘远大于逻辑大小。这是设计里最容易搞反方向的一条，
// 500 个 1 字节文件实测 500 B 逻辑 / 2.0 MB 实际。
func TestStat_SmallFileRoundsUpToBlock(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "tiny")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	info := lstat(t, p)
	bytes, _, _, _, _ := Stat(info)
	if info.Size() != 1 {
		t.Fatalf("逻辑大小应为 1，得到 %d", info.Size())
	}
	if bytes <= uint64(info.Size()) {
		t.Errorf("1 字节文件实际占盘 %d 应远大于逻辑大小 1（块取整）", bytes)
	}
}

// 稀疏文件是反方向：逻辑很大、实际占 0。默认口径必须报实际。
func TestStat_SparseFileReportsNearZero(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "sparse")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	const logical = 64 << 20 // 64 MiB
	// Truncate 而非 Seek：Seek 不会扩展文件，写不出稀疏洞。
	if err := f.Truncate(logical); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	info := lstat(t, p)
	if info.Size() != logical {
		t.Skipf("文件系统未产生预期的稀疏文件（Size=%d）", info.Size())
	}
	bytes, _, _, _, _ := Stat(info)
	if bytes >= logical/2 {
		t.Errorf("稀疏文件实际占盘 %d 应远小于逻辑 %d", bytes, logical)
	}
}

// 硬链接：两条路径同 (dev, ino)、nlink 均为 2。去重就靠这个。
//
// 注意 Nlink 的类型在 darwin(uint16) / linux-arm64(uint32) / linux-amd64(uint64)
// 三向分歧，statOf 里统一转 uint64。类型是否转对是**编译期**属性，靠交叉
// 编译验证（见 TestCrossCompileTargets 的说明）；这条用例验证的是运行期
// 确实读到了正确的值。
func TestStat_HardlinkSharesInodeAndReportsNlink(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 inode 信息")
	}
	dir := t.TempDir()
	orig := filepath.Join(dir, "orig")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(orig, make([]byte, 8192), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(orig, link); err != nil {
		t.Skipf("本文件系统不支持硬链接：%v", err)
	}

	b1, d1, i1, n1, _ := Stat(lstat(t, orig))
	b2, d2, i2, n2, _ := Stat(lstat(t, link))

	if d1 != d2 || i1 != i2 {
		t.Errorf("硬链接应同 (dev,ino)：(%d,%d) vs (%d,%d)", d1, i1, d2, i2)
	}
	if b1 != b2 {
		t.Errorf("硬链接两端占盘应相同：%d vs %d", b1, b2)
	}
	if n1 != 2 || n2 != 2 {
		t.Errorf("nlink 应为 2，得到 %d / %d", n1, n2)
	}
	// 单链接文件 nlink 必须是 1，否则「仅 nlink>1 才进延迟队列」的优化会失效
	solo := filepath.Join(dir, "solo")
	if err := os.WriteFile(solo, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, n, _ := Stat(lstat(t, solo)); n != 1 {
		t.Errorf("单链接文件 nlink 应为 1，得到 %d", n)
	}
}

// 不同目录必须能通过 dev 区分（跨文件系统检测的基础）。同一临时目录下的
// 两个文件 dev 必然相同，这里只验证 dev 被填充且自洽。
func TestStat_DevIsPopulatedAndConsistent(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 dev 信息")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, da, _, _, _ := Stat(lstat(t, a))
	_, db, _, _, _ := Stat(lstat(t, b))
	if da != db {
		t.Errorf("同一文件系统下 dev 应相同：%d vs %d", da, db)
	}
	if da == 0 {
		t.Error("dev 不应为 0（未被填充）")
	}
}

// 符号链接必须用 lstat 量它自己，不能跟随到目标 —— 否则会重复计数并可能绕环。
func TestStat_SymlinkMeasuresLinkNotTarget(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, make([]byte, 1<<20), 0o644); err != nil { // 1 MiB
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("本平台不支持符号链接：%v", err)
	}

	_, _, tIno, _, _ := Stat(lstat(t, target))
	lBytes, _, lIno, _, _ := Stat(lstat(t, link))

	if lIno == tIno {
		t.Error("lstat 符号链接应拿到链接自身的 inode，而非目标的")
	}
	if lBytes >= 1<<20 {
		t.Errorf("符号链接自身占盘 %d 不应接近目标的 1 MiB", lBytes)
	}
}

// 目录自身也占块，必须计入总量 —— 否则 ext4 上与 du 的对拍会失败
// （APFS 上目录 Blocks=0，所以 macOS 看不出这个问题）。
func TestStat_DirectoryHasOwnStat(t *testing.T) {
	if !Supported {
		t.Skip("本平台无 st_blocks")
	}
	dir := t.TempDir()
	bytes, _, ino, _, ok := Stat(lstat(t, dir))
	if !ok {
		t.Fatal("目录 Stat 应成功")
	}
	if ino == 0 {
		t.Error("目录 inode 不应为 0")
	}
	t.Logf("目录自身占盘 = %d 字节（APFS 通常为 0，ext4 通常为 4096）", bytes)
}

// 与系统 du 对拍单个文件的占盘。这是「口径正确」的最直接证据。
func TestStat_MatchesSystemDu(t *testing.T) {
	if !Supported || runtime.GOOS == "windows" {
		t.Skip("本平台无 du 或无 st_blocks")
	}
	if _, err := exec.LookPath("du"); err != nil {
		t.Skip("找不到 du")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, make([]byte, 100000), 0o644); err != nil {
		t.Fatal(err)
	}

	// du -k 输出 KiB（1024 字节块）
	out, err := exec.Command("du", "-k", p).Output()
	if err != nil {
		t.Skipf("du 执行失败：%v", err)
	}
	field := strings.Fields(string(out))
	if len(field) == 0 {
		t.Skip("du 输出为空")
	}
	duKiB, err := strconv.ParseUint(field[0], 10, 64)
	if err != nil {
		t.Skipf("解析 du 输出失败：%v", err)
	}

	bytes, _, _, _, _ := Stat(lstat(t, p))
	if bytes != duKiB*1024 {
		t.Errorf("占盘 %d 字节与 du -k 的 %d KiB (=%d 字节) 不符", bytes, duKiB, duKiB*1024)
	}
}
