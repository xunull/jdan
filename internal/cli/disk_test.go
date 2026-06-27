package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/diskx"
)

func fakeMounts() []diskx.Mount {
	return []diskx.Mount{
		{Device: "/dev/disk1", Mountpoint: "/", Fstype: "apfs", BlockSize: 1024, Blocks: 1000, Bfree: 200, Bavail: 150, Files: 100, Ffree: 40},
		{Device: "devfs", Mountpoint: "/dev", Fstype: "devfs", BlockSize: 1024, Blocks: 50, Bfree: 0, Bavail: 0},
		{Device: "com.apple.TimeMachine.2026-06-27-064158.local@/dev/disk3s5", Mountpoint: "/Volumes/com.apple.TimeMachine.localsnapshots/x/Data", Fstype: "apfs", BlockSize: 1024, Blocks: 1000, Bfree: 200, Bavail: 150},
	}
}

func runDisk(t *testing.T, deps diskCmdDeps, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	deps.out = &buf
	if deps.mounts == nil {
		deps.mounts = func() ([]diskx.Mount, error) { return fakeMounts(), nil }
	}
	cmd := newDiskCommand(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestDiskCmd_Default(t *testing.T) {
	out, err := runDisk(t, diskCmdDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/dev/disk1") || !strings.Contains(out, "文件系统") {
		t.Errorf("默认应列真实挂载:\n%s", out)
	}
	// 默认隐藏 devfs
	if strings.Contains(out, "devfs") {
		t.Errorf("默认应隐藏伪文件系统 devfs:\n%s", out)
	}
	// 默认隐藏 TimeMachine 本地快照
	if strings.Contains(out, "com.apple.TimeMachine") {
		t.Errorf("默认应隐藏 TM 本地快照:\n%s", out)
	}
}

func TestDiskCmd_All(t *testing.T) {
	out, err := runDisk(t, diskCmdDeps{}, "-a")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "devfs") {
		t.Errorf("-a 应显示 devfs:\n%s", out)
	}
	if !strings.Contains(out, "com.apple.TimeMachine") {
		t.Errorf("-a 应显示 TM 本地快照:\n%s", out)
	}
}

func TestDiskCmd_Path(t *testing.T) {
	called := ""
	deps := diskCmdDeps{
		statPath: func(p string) (diskx.Mount, error) {
			called = p
			return diskx.Mount{Device: "/dev/diskX", Mountpoint: p, BlockSize: 1024, Blocks: 500, Bfree: 100, Bavail: 100}, nil
		},
	}
	out, err := runDisk(t, deps, "/data")
	if err != nil {
		t.Fatal(err)
	}
	if called != "/data" {
		t.Errorf("应调用 statPath(/data)，得到 %q", called)
	}
	if !strings.Contains(out, "/dev/diskX") {
		t.Errorf("单路径输出错:\n%s", out)
	}
}

func TestDiskCmd_PathError(t *testing.T) {
	deps := diskCmdDeps{
		statPath: func(string) (diskx.Mount, error) { return diskx.Mount{}, errors.New("no such file") },
	}
	if _, err := runDisk(t, deps, "/nope"); err == nil {
		t.Error("路径不存在应报错")
	}
}

func TestDiskCmd_JSON(t *testing.T) {
	out, err := runDisk(t, diskCmdDeps{}, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var v []map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json 应合法:\n%s", out)
	}
	if len(v) == 0 || v[0]["mountpoint"] != "/" {
		t.Errorf("json 内容错: %+v", v)
	}
}

func TestDiskCmd_NoANSIWhenPiped(t *testing.T) {
	out, err := runDisk(t, diskCmdDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "\x1b") {
		t.Error("管道（非 TTY）不应有 ANSI")
	}
}

func TestDiskCmd_MountsError(t *testing.T) {
	deps := diskCmdDeps{mounts: func() ([]diskx.Mount, error) { return nil, errors.New("unsupported") }}
	if _, err := runDisk(t, deps); err == nil {
		t.Error("采集失败应报错")
	}
}
