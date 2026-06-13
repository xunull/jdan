package netcheck

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"testing"
)

func TestParseLsofOutput_Empty(t *testing.T) {
	if got := parseLsofOutput(""); len(got) != 0 {
		t.Errorf("empty input → %d listeners, want 0", len(got))
	}
}

func TestParseLsofOutput_OneIPv4Listener(t *testing.T) {
	out := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
jdan    12345 quincy   6u  IPv4 0x12345678901234        0t0  TCP *:8080 (LISTEN)
`
	listeners := parseLsofOutput(out)
	if len(listeners) != 1 {
		t.Fatalf("got %d, want 1", len(listeners))
	}
	l := listeners[0]
	if l.Process != "jdan" {
		t.Errorf("process %q", l.Process)
	}
	if l.PID != 12345 {
		t.Errorf("pid %d", l.PID)
	}
	if l.User != "quincy" {
		t.Errorf("user %q", l.User)
	}
	if l.Bind != "*:8080" {
		t.Errorf("bind %q", l.Bind)
	}
	if l.Proto != "tcp" {
		t.Errorf("proto %q", l.Proto)
	}
}

func TestParseLsofOutput_IPv6(t *testing.T) {
	out := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
chrome   9999 alice  15u  IPv6 0xabc                  0t0  TCP [::1]:8080 (LISTEN)
`
	listeners := parseLsofOutput(out)
	if len(listeners) != 1 {
		t.Fatal("expected 1 listener")
	}
	if listeners[0].Proto != "tcp6" {
		t.Errorf("ipv6 proto: %q", listeners[0].Proto)
	}
	if listeners[0].Bind != "[::1]:8080" {
		t.Errorf("bind %q", listeners[0].Bind)
	}
}

func TestParseLsofOutput_Multiple(t *testing.T) {
	out := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
jdan    100   alice   6u  IPv4 0x1                   0t0  TCP *:8080 (LISTEN)
node    200   bob    15u  IPv4 0x2                   0t0  TCP 127.0.0.1:8080 (LISTEN)
`
	listeners := parseLsofOutput(out)
	if len(listeners) != 2 {
		t.Fatalf("got %d, want 2", len(listeners))
	}
}

func TestParseLsofOutput_SkipsMalformed(t *testing.T) {
	out := `COMMAND   PID  USER   FD   TYPE             DEVICE SIZE/OFF NODE NAME
malformed line here
jdan    100   alice   6u  IPv4 0x1                   0t0  TCP *:8080 (LISTEN)
`
	listeners := parseLsofOutput(out)
	if len(listeners) != 1 {
		t.Errorf("got %d listeners, want 1 (malformed should skip)", len(listeners))
	}
}

func TestListener_IsLANReachable(t *testing.T) {
	// IPv6 语义：:: 是 all-interfaces（v4 的 0.0.0.0）；::1 是 loopback
	for _, tc := range []struct {
		bind string
		want bool
	}{
		{"*:8080", true},               // lsof IPv4 all-interfaces
		{"0.0.0.0:8080", true},
		{"::1:8080", false},            // IPv6 loopback
		{"127.0.0.1:8080", false},
		{"192.168.1.42:8080", true},    // 具体 LAN IP
		{"10.0.0.5:8080", true},
	} {
		l := Listener{Bind: tc.bind}
		if got := l.IsLANReachable(); got != tc.want {
			t.Errorf("bind=%q: got %v, want %v", tc.bind, got, tc.want)
		}
	}
}

func TestFindListeners_LsofNotInstalled(t *testing.T) {
	orig := LsofRunCmd
	defer func() { LsofRunCmd = orig }()

	LsofRunCmd = func(ctx context.Context, port int) ([]byte, error) {
		return nil, &exec.Error{Name: "lsof", Err: exec.ErrNotFound}
	}

	_, err := FindListeners(context.Background(), 8080)
	if !errors.Is(err, ErrLsofNotInstalled) {
		t.Errorf("got %v, want ErrLsofNotInstalled", err)
	}
}

func TestFindListeners_NoMatch(t *testing.T) {
	orig := LsofRunCmd
	defer func() { LsofRunCmd = orig }()

	// lsof 没有结果时 exit code = 1 + 空 stdout
	LsofRunCmd = func(ctx context.Context, port int) ([]byte, error) {
		return nil, &exitErr{code: 1}
	}

	listeners, err := FindListeners(context.Background(), 8080)
	if err != nil {
		t.Errorf("no-match should be silent nil, got: %v", err)
	}
	if len(listeners) != 0 {
		t.Errorf("expected 0 listeners, got %d", len(listeners))
	}
}

func TestFindListeners_RealLocalSocket(t *testing.T) {
	// 实际起一个 server，验证 lsof 能看到
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port
	got, err := FindListeners(context.Background(), port)
	if err != nil {
		if errors.Is(err, ErrLsofNotInstalled) {
			t.Skip("lsof not installed")
		}
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Errorf("expected to find listener on port %d, got none", port)
	}
	// 至少一个 listener 应当是我们的 test binary
	foundOurs := false
	for _, lst := range got {
		if strings.Contains(lst.Bind, "127.0.0.1") || strings.Contains(lst.Bind, "*:") {
			foundOurs = true
		}
	}
	if !foundOurs {
		t.Errorf("our listener not in lsof output: %+v", got)
	}
}

// exitErr 模拟 *exec.ExitError 的最小子集（只 ExitCode）
type exitErr struct {
	code int
}

func (e *exitErr) Error() string { return "exit status " + string(rune('0'+e.code)) }
func (e *exitErr) ExitCode() int { return e.code }
