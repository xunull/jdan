package httpserve

import (
	"errors"
	"net"
	"testing"
)

// fakeListener 模拟 net.Listener，记录端口避免真的占用资源
type fakeListener struct {
	port int
}

func (f *fakeListener) Accept() (net.Conn, error) { return nil, errors.New("not used") }
func (f *fakeListener) Close() error              { return nil }
func (f *fakeListener) Addr() net.Addr {
	return &net.TCPAddr{Port: f.port}
}

func TestFindFreePort_RejectsNegative(t *testing.T) {
	_, err := FindFreePort(-1)
	if err == nil {
		t.Error("negative prefer should error")
	}
}

func TestFindFreePort_PreferZero_UsesRandom(t *testing.T) {
	calls := 0
	mock := func(addr string) (net.Listener, error) {
		calls++
		// 期望调用形式 ":0"
		if addr != ":0" {
			t.Errorf("prefer=0 should call listen(\":0\"), got %s", addr)
		}
		return &fakeListener{port: 54321}, nil
	}
	port, err := findFreePortWith(0, mock)
	if err != nil {
		t.Fatal(err)
	}
	if port != 54321 {
		t.Errorf("got port %d, want 54321 from mock", port)
	}
	if calls != 1 {
		t.Errorf("expected 1 listen call, got %d", calls)
	}
}

func TestFindFreePort_PreferUsedFirst(t *testing.T) {
	mock := func(addr string) (net.Listener, error) {
		// :8080 应当被首先尝试，返回成功
		if addr == ":8080" {
			return &fakeListener{port: 8080}, nil
		}
		return nil, errors.New("nope")
	}
	port, err := findFreePortWith(8080, mock)
	if err != nil {
		t.Fatal(err)
	}
	if port != 8080 {
		t.Errorf("prefer=8080 available, got %d", port)
	}
}

func TestFindFreePort_FallbackOnPreferTaken(t *testing.T) {
	// 8080 和 8081 都被占用，8082 成功
	mock := func(addr string) (net.Listener, error) {
		switch addr {
		case ":8080", ":8081":
			return nil, errors.New("in use")
		case ":8082":
			return &fakeListener{port: 8082}, nil
		}
		return nil, errors.New("not in test")
	}
	port, err := findFreePortWith(8080, mock)
	if err != nil {
		t.Fatal(err)
	}
	if port != 8082 {
		t.Errorf("expected fallback to 8082, got %d", port)
	}
}

func TestFindFreePort_RandomFallbackAfterRangeExhausted(t *testing.T) {
	// prefer 范围全失败，应回退到 random ":0"
	mock := func(addr string) (net.Listener, error) {
		if addr == ":0" {
			return &fakeListener{port: 60000}, nil
		}
		return nil, errors.New("in use")
	}
	port, err := findFreePortWith(8080, mock)
	if err != nil {
		t.Fatal(err)
	}
	if port != 60000 {
		t.Errorf("expected random fallback to 60000, got %d", port)
	}
}

func TestFindFreePort_AllFail(t *testing.T) {
	mock := func(addr string) (net.Listener, error) {
		return nil, errors.New("system out of ports")
	}
	if _, err := findFreePortWith(8080, mock); err == nil {
		t.Error("when everything fails, should return error")
	}
}

// 真接口：保证基本能跑通（CI 上端口偶尔被 sidecar 占用，所以不强求 8080）
func TestFindFreePort_RealNetListen(t *testing.T) {
	port, err := FindFreePort(0)
	if err != nil {
		t.Fatalf("real listen failed: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Errorf("port %d out of range", port)
	}
}
