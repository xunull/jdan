package diskx

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
)

func TestHumanBytes(t *testing.T) {
	cases := map[uint64]string{
		0:             "0B",
		1023:          "1023B",
		1024:          "1.0Ki",
		1536:          "1.5Ki",
		14 * 1 << 30:  "14Gi",
		1<<40 + 1<<39: "1.5Ti",
		926 * 1 << 30: "926Gi",
	}
	for n, want := range cases {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %s, want %s", n, got, want)
		}
	}
}

func TestUsePercent_DfAligned(t *testing.T) {
	// 总 100 块、空闲 20（含 root 保留）、非 root 可用 10 → 已用 80，分母 90 → ceil(8000/90)=89
	m := Mount{BlockSize: 1, Blocks: 100, Bfree: 20, Bavail: 10}
	if got := m.UsePercent(); got != 89 {
		t.Errorf("UsePercent = %d, want 89 (df 公式：used/(used+avail) 向上取整)", got)
	}
	// 满盘
	full := Mount{BlockSize: 1, Blocks: 100, Bfree: 0, Bavail: 0}
	if got := full.UsePercent(); got != 100 {
		t.Errorf("full UsePercent = %d, want 100", got)
	}
}

func TestUsePercent_ZeroNoPanic(t *testing.T) {
	if got := (Mount{}).UsePercent(); got != 0 {
		t.Errorf("zero mount UsePercent = %d, want 0", got)
	}
}

func TestInodePercent(t *testing.T) {
	m := Mount{Files: 100, Ffree: 25}
	if got := m.InodePercent(); got != 75 {
		t.Errorf("InodePercent = %d, want 75", got)
	}
	if got := (Mount{}).InodePercent(); got != 0 {
		t.Errorf("zero inodes InodePercent = %d, want 0", got)
	}
}

func TestFilter(t *testing.T) {
	mounts := []Mount{
		{Device: "/dev/disk1", Fstype: "apfs", BlockSize: 1, Blocks: 100},
		{Device: "devfs", Fstype: "devfs", BlockSize: 1, Blocks: 100},
		{Device: "tmpfs", Fstype: "tmpfs", BlockSize: 1, Blocks: 100},
		{Device: "empty", Fstype: "apfs", BlockSize: 1, Blocks: 0}, // 0 容量
	}
	got := Filter(mounts, false)
	if len(got) != 1 || got[0].Device != "/dev/disk1" {
		t.Errorf("Filter 应只留真实非空卷，得到 %+v", got)
	}
	if len(Filter(mounts, true)) != 4 {
		t.Error("Filter(all=true) 应全留")
	}
}

func TestFilter_HidesLocalSnapshots(t *testing.T) {
	mounts := []Mount{
		{Device: "/dev/disk1", Fstype: "apfs", BlockSize: 1, Blocks: 100, Mountpoint: "/"},
		// 按设备名前缀识别的 TM 本地快照
		{Device: "com.apple.TimeMachine.2026-06-27-064158.local@/dev/disk3s5", Fstype: "apfs", BlockSize: 1, Blocks: 100, Mountpoint: "/Volumes/x"},
		// 按挂载点前缀识别的 TM 本地快照
		{Device: "/dev/disk3s5", Fstype: "apfs", BlockSize: 1, Blocks: 100, Mountpoint: "/Volumes/com.apple.TimeMachine.localsnapshots/Backups.backupdb/m1max/2026-06-27-064158/Data"},
	}
	got := Filter(mounts, false)
	if len(got) != 1 || got[0].Device != "/dev/disk1" {
		t.Errorf("默认应隐藏 TM 本地快照（设备名/挂载点两种识别），得到 %+v", got)
	}
	if len(Filter(mounts, true)) != 3 {
		t.Error("-a 应显示快照")
	}
}

func TestBar(t *testing.T) {
	if b := bar(0, 9); b != strings.Repeat("░", 9) {
		t.Errorf("bar(0) = %q", b)
	}
	if b := bar(100, 9); b != strings.Repeat("█", 9) {
		t.Errorf("bar(100) = %q", b)
	}
	// 50% × 9 → round(4.5)=5 满格
	if filled := strings.Count(bar(50, 9), "█"); filled != 5 {
		t.Errorf("bar(50,9) filled = %d, want 5", filled)
	}
}

func TestColorize(t *testing.T) {
	if s := colorize("98%", 98, true); !strings.Contains(s, "\x1b[31m") {
		t.Error("≥90% 应染红")
	}
	if s := colorize("80%", 80, true); !strings.Contains(s, "\x1b[33m") {
		t.Error("≥75% 应染黄")
	}
	if s := colorize("50%", 50, true); strings.Contains(s, "\x1b") {
		t.Error("<75% 不染色")
	}
	if s := colorize("98%", 98, false); strings.Contains(s, "\x1b") {
		t.Error("color=false 不应有 ANSI")
	}
}

func TestRender(t *testing.T) {
	mounts := []Mount{
		{Device: "/dev/disk1", Mountpoint: "/", Fstype: "apfs", BlockSize: 1024, Blocks: 1000, Bfree: 200, Bavail: 200, Files: 100, Ffree: 50},
	}
	out := Render(mounts, RenderOptions{})
	for _, want := range []string{"文件系统", "容量", "挂载点", "/dev/disk1", "/"} {
		if !strings.Contains(out, want) {
			t.Errorf("Render 缺 %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "\x1b") {
		t.Error("Color=false 不应有 ANSI")
	}
}

func TestRender_Inodes(t *testing.T) {
	mounts := []Mount{{Device: "/dev/disk1", Mountpoint: "/", BlockSize: 1024, Blocks: 1000, Files: 100, Ffree: 50}}
	out := Render(mounts, RenderOptions{Inodes: true})
	if !strings.Contains(out, "Inode") {
		t.Errorf("inode 模式表头应有 Inode:\n%s", out)
	}
}

func TestRender_Bytes(t *testing.T) {
	mounts := []Mount{{Device: "/dev/disk1", Mountpoint: "/", BlockSize: 1024, Blocks: 1000, Bfree: 200, Bavail: 200}}
	out := Render(mounts, RenderOptions{Bytes: true})
	if !strings.Contains(out, "1024000") { // 1000*1024 raw
		t.Errorf("--bytes 应显示原始字节:\n%s", out)
	}
}

func TestVisWidth_IgnoresANSI(t *testing.T) {
	if w := visWidth("\x1b[31m86%\x1b[0m"); w != 3 {
		t.Errorf("ANSI-wrapped '86%%' visual width = %d, want 3", w)
	}
}

// 回归：CJK locale 下 runewidth 默认把 █ 判成 2 列（ambiguous→wide），与终端渲染（1 列）
// 不符，导致整列错位。diskx 用 narrow 条件锁死按 1 列算；中文宽字符仍按 2 列。
func TestVisWidth_AmbiguousBlocksNarrowUnderCJK(t *testing.T) {
	old := runewidth.DefaultCondition.EastAsianWidth
	runewidth.DefaultCondition.EastAsianWidth = true // 模拟 zh_CN.UTF-8 终端
	defer func() { runewidth.DefaultCondition.EastAsianWidth = old }()

	if w := visWidth("█"); w != 1 {
		t.Errorf("█ (U+2588) 应按 1 列测量，得到 %d（CJK locale 回归）", w)
	}
	if w := visWidth("░"); w != 1 {
		t.Errorf("░ (U+2591) 应按 1 列测量，得到 %d", w)
	}
	if w := visWidth("容"); w != 2 {
		t.Errorf("中文宽字符「容」应仍按 2 列，得到 %d", w)
	}
}

func TestTruncMiddle(t *testing.T) {
	s := "com.apple.TimeMachine.2026-06-27-064158.local@/dev/disk3s5"
	out := truncMiddle(s, 20)
	if visWidth(out) > 20 {
		t.Errorf("截断后宽度 %d > 20: %q", visWidth(out), out)
	}
	if !strings.Contains(out, "…") {
		t.Errorf("应含省略号: %q", out)
	}
	if !strings.HasPrefix(out, "com") {
		t.Errorf("应保留头部: %q", out)
	}
	if !strings.HasSuffix(out, "disk3s5") {
		t.Errorf("应保留尾部: %q", out)
	}
	if truncMiddle("short", 20) != "short" {
		t.Error("未超宽应原样返回")
	}
}

// 长设备名/挂载点在窄终端按终端宽度中间省略号截断；MaxWidth=0（管道/--json/--no-trunc）全显。
func TestRender_TruncatesWhenNarrow(t *testing.T) {
	ms := []Mount{
		{Device: "com.apple.TimeMachine.2026-06-27-064158.local@/dev/disk3s5", Mountpoint: "/Volumes/com.apple.TimeMachine.localsnapshots/Backups.backupdb/m1max/2026-06-27-064158/Data", BlockSize: 1, Blocks: 100, Bfree: 14, Bavail: 14},
	}
	narrow := Render(ms, RenderOptions{MaxWidth: 70})
	if !strings.Contains(narrow, "…") {
		t.Errorf("窄终端应截断:\n%s", narrow)
	}
	for ln := range strings.SplitSeq(strings.TrimRight(narrow, "\n"), "\n") {
		if visWidth(ln) > 70 {
			t.Errorf("行可见宽度 %d > MaxWidth 70: %q", visWidth(ln), ln)
		}
	}
	full := Render(ms, RenderOptions{MaxWidth: 0})
	if strings.Contains(full, "…") {
		t.Errorf("MaxWidth=0 不应截断:\n%s", full)
	}
	if !strings.Contains(full, "/dev/disk3s5") {
		t.Errorf("MaxWidth=0 应保留完整设备名:\n%s", full)
	}
}

// 回归：百分比右对齐到固定 4 宽，条形左缘才能成竖列。
func TestRender_PercentRightAligned(t *testing.T) {
	mounts := []Mount{
		{Device: "a", Mountpoint: "/a", BlockSize: 1, Blocks: 100, Bfree: 96, Bavail: 96}, // 4%
		{Device: "b", Mountpoint: "/b", BlockSize: 1, Blocks: 100, Bfree: 0, Bavail: 0},   // 100%
	}
	out := Render(mounts, RenderOptions{})
	if !strings.Contains(out, "  4%") {
		t.Errorf("4%% 应右对齐成「  4%%」:\n%s", out)
	}
	if !strings.Contains(out, "100%") {
		t.Errorf("100%% 应出现:\n%s", out)
	}
}

// ---- 平台采集 smoke（真机，宽松）----

func TestMounts_Smoke(t *testing.T) {
	ms, err := Mounts()
	if err != nil {
		t.Skipf("Mounts 在此平台不支持：%v", err)
	}
	if len(ms) == 0 {
		t.Skip("无挂载点")
	}
	any := false
	for _, m := range ms {
		if m.Total() > 0 {
			any = true
			break
		}
	}
	if !any {
		t.Error("至少应有一个非零容量的挂载点")
	}
}

func TestStatPath_Root(t *testing.T) {
	m, err := StatPath("/")
	if err != nil {
		t.Skipf("StatPath 在此平台不支持：%v", err)
	}
	if m.Total() == 0 {
		t.Error("根文件系统总容量应 > 0")
	}
}
