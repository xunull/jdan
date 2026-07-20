// Package netcheck 实现 jdan net selfcheck 的服务端自检：
//
//	"我作为 server 该不该被外部访问到？"
//
// 信息来源：firewall（sysprobe）、网络接口（sysprobe）、本机端口监听（lsof exec）、
// 自连接（net.Dial 测自己）。
package netcheck

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Listener 是一个被 lsof 检测到的本地监听 socket。
type Listener struct {
	Process string `json:"process"` // 进程名（comm）
	PID     int    `json:"pid"`
	User    string `json:"user"`
	Bind    string `json:"bind"`           // "*:8080" / "127.0.0.1:8080" / "::1:8080" 等
	Proto   string `json:"proto"`          // tcp / tcp6
	Path    string `json:"path,omitempty"` // 可执行路径（lsof 不直接给，留 future）
}

// IsLANReachable 返回 true 表示 bind 地址可被 LAN 访问。
// 0.0.0.0 / ::（all interfaces）算 LAN-reachable，127.0.0.1 / ::1 只 localhost。
func (l Listener) IsLANReachable() bool {
	b := l.Bind
	// 去掉 port，看 IP
	if i := strings.LastIndex(b, ":"); i >= 0 {
		b = b[:i]
	}
	switch b {
	case "*", "0.0.0.0", "::", "":
		return true
	case "127.0.0.1", "::1", "localhost":
		return false
	}
	// 其他具体 IP（比如 192.168.1.42）也算 LAN-reachable
	return true
}

// LsofRunCmd 是可替换的 lsof exec 函数（测试用）。
var LsofRunCmd = defaultLsofRunCmd

func defaultLsofRunCmd(ctx context.Context, port int) ([]byte, error) {
	// -i :PORT  过滤端口
	// -P        不解析端口名（80→http）
	// -n        不解析 IP（dns lookup 避免）
	// -F pcun0  -F 字段输出：p=PID c=comm u=user n=name 0=null-terminator
	//           我们直接走默认 column 输出，更容易解析
	cmd := exec.CommandContext(ctx, "lsof", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-P", "-n")
	return cmd.Output()
}

// ErrLsofNotInstalled 用来在调用方区分"lsof 缺失" vs 其他错误。
var ErrLsofNotInstalled = errors.New("lsof not installed or not in PATH")

// exitCoder 是被 *exec.ExitError 满足的最小接口，让测试可以注入轻量 mock。
type exitCoder interface {
	ExitCode() int
}

// FindListeners 用 lsof 查指定端口的 LISTEN socket。
// 没人监听返回空 slice + nil。
// lsof 不存在返回 ErrLsofNotInstalled 让上层降级显示。
func FindListeners(ctx context.Context, port int) ([]Listener, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := LsofRunCmd(ctx, port)
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, ErrLsofNotInstalled
		}
		// lsof 在没匹配时退出 1 + 空输出 —— 这是 normal "no listeners"，不是错误
		if ec, ok := err.(exitCoder); ok && ec.ExitCode() == 1 && len(out) == 0 {
			return nil, nil
		}
		return nil, fmt.Errorf("lsof failed: %w", err)
	}
	return parseLsofOutput(string(out)), nil
}

// parseLsofOutput 解析 lsof 默认 column 格式输出。例：
//
//	COMMAND   PID  USER  FD  TYPE             DEVICE SIZE/OFF NODE NAME
//	jdan    12345  alice  6u  IPv4 0xabcd1234...      0t0  TCP *:8080 (LISTEN)
//	Chrome   9999  alice 15u  IPv6 0x123...           0t0  TCP [::1]:8080 (LISTEN)
//
// 我们抽 COMMAND/PID/USER/NAME 字段。
func parseLsofOutput(out string) []Listener {
	var listeners []Listener
	scanner := bufio.NewScanner(strings.NewReader(out))
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	headerSeen := false
	for scanner.Scan() {
		line := scanner.Text()
		if !headerSeen {
			if strings.HasPrefix(line, "COMMAND") {
				headerSeen = true
			}
			continue
		}
		// 拆字段（lsof 用空白分隔；NAME 可能含空格，最后处理）
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		// fields: COMMAND PID USER FD TYPE DEVICE SIZE/OFF NODE NAME...
		comm := fields[0]
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		user := fields[2]
		proto := "tcp"
		if strings.HasPrefix(strings.ToLower(fields[4]), "ipv6") {
			proto = "tcp6"
		}
		// NAME 部分：fields[8:] join，去掉末尾的 (LISTEN)
		name := strings.Join(fields[8:], " ")
		name = strings.TrimSuffix(name, "(LISTEN)")
		name = strings.TrimSpace(name)
		// 形态可能是 *:8080 或 [::1]:8080 或 127.0.0.1:8080
		// 直接保留原文，上层用 IsLANReachable 判断

		listeners = append(listeners, Listener{
			Process: comm,
			PID:     pid,
			User:    user,
			Bind:    name,
			Proto:   proto,
		})
	}
	return listeners
}
