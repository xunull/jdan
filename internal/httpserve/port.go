package httpserve

import (
	"fmt"
	"net"
)

// FindFreePort 找一个可以 bind 的 TCP 端口。
//
// 策略：
//   - prefer > 0 → 试 [prefer, prefer+50] 这个区间，第一个成功的返回；都失败回退到 random
//   - prefer == 0 → 直接让内核分配（net.Listen(":0")）
//   - prefer < 0 → 视为非法，返回错误
//
// 把这个逻辑放在 listenFunc 抽象后面，单元测试可以注入 mock。
func FindFreePort(prefer int) (int, error) {
	return findFreePortWith(prefer, defaultListen)
}

type listenFunc func(addr string) (net.Listener, error)

func defaultListen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func findFreePortWith(prefer int, listen listenFunc) (int, error) {
	if prefer < 0 {
		return 0, fmt.Errorf("invalid preferred port %d", prefer)
	}
	if prefer > 0 {
		for p := prefer; p <= prefer+50 && p <= 65535; p++ {
			if ok, port := tryListen(listen, p); ok {
				return port, nil
			}
		}
	}
	// random fallback
	if ok, port := tryListen(listen, 0); ok {
		return port, nil
	}
	return 0, fmt.Errorf("no free port available")
}

func tryListen(listen listenFunc, port int) (bool, int) {
	addr := fmt.Sprintf(":%d", port)
	l, err := listen(addr)
	if err != nil {
		return false, 0
	}
	defer l.Close()
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		return false, 0
	}
	return true, tcpAddr.Port
}
