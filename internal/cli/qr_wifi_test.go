package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func runQRWifi(t *testing.T, in io.Reader, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := newQRWifiCommand(qrWifiCmdDeps{out: &out, in: in})
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	return out.String(), err
}

func TestQRWifi_JSON(t *testing.T) {
	out, err := runQRWifi(t, nil, "MyNet", "-p", "s3cr3t", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("非法 JSON: %v\n%s", err, out)
	}
	if got["ssid"] != "MyNet" || got["payload"] != "WIFI:T:WPA;S:MyNet;P:s3cr3t;;" {
		t.Errorf("ssid/payload 错: %v", got)
	}
}

func TestQRWifi_PositionalAndFlagConflict(t *testing.T) {
	_, err := runQRWifi(t, nil, "MyNet", "--ssid", "Other", "-p", "x")
	if err == nil || !strings.Contains(err.Error(), "二选一") {
		t.Errorf("位置参数 + --ssid 应冲突报错，got %v", err)
	}
}

func TestQRWifi_PasswordStdin(t *testing.T) {
	out, err := runQRWifi(t, strings.NewReader("pw with space\n"), "Home", "--password-stdin", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "P:pw with space;") {
		t.Errorf("stdin 密码（含空格）未正确进入 payload:\n%s", out)
	}
}

func TestQRWifi_BothPasswordSourcesConflict(t *testing.T) {
	_, err := runQRWifi(t, strings.NewReader("x"), "Home", "-p", "y", "--password-stdin")
	if err == nil || !strings.Contains(err.Error(), "二选一") {
		t.Errorf("-p 与 --password-stdin 应冲突，got %v", err)
	}
}

func TestQRWifi_TerminalRenders(t *testing.T) {
	out, err := runQRWifi(t, nil, "MyNet", "-p", "x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "█") {
		t.Errorf("终端应渲染二维码块字符，got %q", out[:min(40, len(out))])
	}
}
