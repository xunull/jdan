package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runGrab(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newGrabCommand(grabCmdDeps{out: &out, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute() // 先执行再读 buffer（求值顺序坑）
	return out.String(), err
}

func TestGrab_AllTypesLabeled(t *testing.T) {
	in := strings.NewReader("u https://x.com m a@b.com i 1.2.3.4")
	out, err := runGrab(t, in)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"url:   https://x.com", "email: a@b.com", "ip:    1.2.3.4"} {
		if !strings.Contains(out, want) {
			t.Errorf("缺 %q:\n%s", want, out)
		}
	}
}

func TestGrab_SingleTypeUnlabeled(t *testing.T) {
	out, err := runGrab(t, strings.NewReader("a 1.2.3.4 b 8.8.8.8"), "-t", "ip")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "ip:") {
		t.Errorf("单类型不应带标签:\n%s", out)
	}
	if !strings.Contains(out, "1.2.3.4") || !strings.Contains(out, "8.8.8.8") {
		t.Errorf("缺 IP:\n%s", out)
	}
}

func TestGrab_Count(t *testing.T) {
	out, err := runGrab(t, strings.NewReader("1.1.1.1 1.1.1.1 8.8.8.8"), "-t", "ip", "--count")
	if err != nil {
		t.Fatal(err)
	}
	// 1.1.1.1 出现 2 次应排在前
	if !strings.Contains(out, "2  1.1.1.1") {
		t.Errorf("--count 应显示次数:\n%s", out)
	}
	if strings.Index(out, "1.1.1.1") > strings.Index(out, "8.8.8.8") {
		t.Errorf("应按次数降序:\n%s", out)
	}
}

func TestGrab_JSON(t *testing.T) {
	out, err := runGrab(t, strings.NewReader("a@b.com http://x.com 1.2.3.4"), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string][]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("非法 JSON: %v\n%s", err, out)
	}
	if len(got["email"]) != 1 || len(got["url"]) != 1 || len(got["ip"]) != 1 {
		t.Errorf("json 分组错: %v", got)
	}
}

func TestGrab_File(t *testing.T) {
	p := filepath.Join(t.TempDir(), "log.txt")
	if err := os.WriteFile(p, []byte("ip 9.9.9.9 here"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runGrab(t, nil, "-t", "ip", p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "9.9.9.9") {
		t.Errorf("文件输入未抽到:\n%s", out)
	}
}

func TestGrab_LiteralText(t *testing.T) {
	// 参数不是文件 → 当字面文本
	out, err := runGrab(t, nil, "-t", "email", "contact me@here.io now")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "me@here.io") {
		t.Errorf("字面文本未抽到:\n%s", out)
	}
}

func TestGrab_UnknownType(t *testing.T) {
	_, err := runGrab(t, strings.NewReader("x"), "-t", "phone")
	if err == nil || !strings.Contains(err.Error(), "未知类型") {
		t.Errorf("未知类型应报错，got %v", err)
	}
}
