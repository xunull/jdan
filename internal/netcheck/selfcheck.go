package netcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/xunull/jdan/internal/sysprobe"
)

// Options 控制一次 SelfCheck 的范围。
type Options struct {
	Port    int           // 0 = 不查具体端口
	Timeout time.Duration // 自连接超时；默认 1s
}

// Report 是 SelfCheck 的完整输出。
type Report struct {
	OS         string            `json:"os"`
	Arch       string            `json:"arch"`
	Firewall   FirewallSection   `json:"firewall"`
	Interfaces []InterfaceInfo   `json:"interfaces"`
	Listening  *ListeningSection `json:"listening,omitempty"`
	SelfLoop   *SelfLoopSection  `json:"self_loop,omitempty"`
	Prediction string            `json:"prediction"`
}

type FirewallSection struct {
	Detected bool   `json:"detected"`
	State    string `json:"state"` // "enabled" / "disabled" / "unknown"
	Hint     string `json:"hint,omitempty"`
}

type InterfaceInfo struct {
	Name     string   `json:"name"`
	IPs      []string `json:"ips"`
	LAN      bool     `json:"lan"` // 至少有一个 RFC1918 地址
	Loopback bool     `json:"loopback"`
	Primary  bool     `json:"primary"` // 是默认路由出口（best-effort 判断）
}

type ListeningSection struct {
	Port        int        `json:"port"`
	Listeners   []Listener `json:"listeners"`
	LsofMissing bool       `json:"lsof_missing,omitempty"`
}

type SelfLoopSection struct {
	Port       int           `json:"port"`
	Localhost  ConnectResult `json:"localhost"`
	PrimaryLAN ConnectResult `json:"primary_lan,omitempty"`
}

type ConnectResult struct {
	Addr     string        `json:"addr"`
	Success  bool          `json:"success"`
	Duration time.Duration `json:"duration_ns"`
	Err      string        `json:"error,omitempty"`
}

// SelfCheck 主入口：综合检测本机网络/防火墙/监听状态，并预测外部访问情况。
func SelfCheck(ctx context.Context, opts Options) (*Report, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 1 * time.Second
	}

	r := &Report{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// firewall
	r.Firewall = checkFirewall()

	// interfaces
	r.Interfaces = describeInterfaces()

	// listening (optional)
	if opts.Port > 0 {
		r.Listening = checkListening(ctx, opts.Port)
		// self-loop
		r.SelfLoop = runSelfLoop(ctx, opts.Port, opts.Timeout, r.Interfaces)
	}

	r.Prediction = buildPrediction(r)
	return r, nil
}

func checkFirewall() FirewallSection {
	if runtime.GOOS != "darwin" {
		return FirewallSection{
			Detected: false,
			State:    "unknown",
			Hint:     "firewall check on " + runtime.GOOS + " not implemented (PRs welcome)",
		}
	}
	state := sysprobe.MacFirewallState()
	switch state {
	case sysprobe.FirewallEnabled:
		return FirewallSection{
			Detected: true,
			State:    "enabled",
			Hint: "macOS App Firewall is ON. unsigned binaries (like jdan) may be blocked.\n" +
				"  fix:\n" +
				"    sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add $(which jdan)\n" +
				"    sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp $(which jdan)",
		}
	case sysprobe.FirewallDisabled:
		return FirewallSection{Detected: true, State: "disabled"}
	default:
		return FirewallSection{Detected: false, State: "unknown"}
	}
}

func describeInterfaces() []InterfaceInfo {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	// best-effort: 哪个接口是默认路由出口
	primaryIfaceName := guessPrimaryInterface()

	var result []InterfaceInfo
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		info := InterfaceInfo{
			Name:     iface.Name,
			Loopback: iface.Flags&net.FlagLoopback != 0,
			Primary:  iface.Name == primaryIfaceName,
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			info.IPs = append(info.IPs, addr.String())
			if v4 := ip.To4(); v4 != nil {
				if isPrivateV4(v4) {
					info.LAN = true
				}
			}
		}
		if len(info.IPs) > 0 {
			result = append(result, info)
		}
	}
	return result
}

func isPrivateV4(v4 net.IP) bool {
	switch {
	case v4[0] == 10:
		return true
	case v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31:
		return true
	case v4[0] == 192 && v4[1] == 168:
		return true
	}
	return false
}

// guessPrimaryInterface 用一个 UDP "假 dial" 8.8.8.8:53 来让内核选默认出口
// 接口，看 LocalAddr 是哪个 IP，再反查接口名。不真发包。
func guessPrimaryInterface() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr == nil {
		return ""
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipn, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipn.IP.Equal(localAddr.IP) {
				return iface.Name
			}
		}
	}
	return ""
}

func checkListening(ctx context.Context, port int) *ListeningSection {
	listeners, err := FindListeners(ctx, port)
	s := &ListeningSection{Port: port}
	if errors.Is(err, ErrLsofNotInstalled) {
		s.LsofMissing = true
		return s
	}
	if err == nil {
		s.Listeners = listeners
	}
	return s
}

func runSelfLoop(ctx context.Context, port int, timeout time.Duration, ifaces []InterfaceInfo) *SelfLoopSection {
	s := &SelfLoopSection{Port: port}
	s.Localhost = tryHTTPGet(ctx, fmt.Sprintf("http://127.0.0.1:%d", port), timeout)

	// 找 primary LAN IP
	for _, iface := range ifaces {
		if iface.Primary && iface.LAN {
			for _, addrStr := range iface.IPs {
				ip, _, err := net.ParseCIDR(addrStr)
				if err != nil {
					continue
				}
				v4 := ip.To4()
				if v4 == nil || !isPrivateV4(v4) {
					continue
				}
				s.PrimaryLAN = tryHTTPGet(ctx, fmt.Sprintf("http://%s:%d", v4, port), timeout)
				return s
			}
		}
	}
	return s
}

func tryHTTPGet(ctx context.Context, url string, timeout time.Duration) ConnectResult {
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, "GET", url, nil)
	if err != nil {
		return ConnectResult{Addr: url, Err: err.Error()}
	}
	start := time.Now()
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		return ConnectResult{Addr: url, Success: false, Duration: dur, Err: err.Error()}
	}
	resp.Body.Close()
	return ConnectResult{Addr: url, Success: true, Duration: dur}
}

func buildPrediction(r *Report) string {
	if r.Listening == nil {
		// 没指定 port，给通用建议
		switch r.Firewall.State {
		case "enabled":
			return "macOS firewall is ON. when you start a server, add jdan to firewall allow list:\n" +
				"  sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add $(which jdan) && \\\n" +
				"  sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp $(which jdan)"
		case "disabled":
			return "no firewall blocking. external clients should be able to reach any listening port."
		}
		return "system reports limited info; check macOS Settings → Network → Firewall manually."
	}

	port := r.Listening.Port
	hasListener := len(r.Listening.Listeners) > 0
	hasLANBind := false
	for _, l := range r.Listening.Listeners {
		if l.IsLANReachable() {
			hasLANBind = true
		}
	}

	if !hasListener {
		if r.Listening.LsofMissing {
			return fmt.Sprintf("can't determine if anyone is listening on :%d (install lsof to enable).", port)
		}
		return fmt.Sprintf("nothing is listening on :%d. start your server first.", port)
	}

	if !hasLANBind {
		return fmt.Sprintf("port :%d is bound to loopback only (127.0.0.1 or ::1).\n"+
			"  external clients CANNOT reach this. server must bind 0.0.0.0 or specific LAN IP.", port)
	}

	if r.Firewall.State == "enabled" {
		return fmt.Sprintf("port :%d is bound LAN-reachable, BUT macOS firewall is on.\n"+
			"  if external clients see 'connection refused', apply firewall fix above.", port)
	}

	if r.SelfLoop != nil && r.SelfLoop.PrimaryLAN.Addr != "" && r.SelfLoop.PrimaryLAN.Success {
		return fmt.Sprintf("port :%d is LAN-reachable from self. external clients on same LAN should reach %s.",
			port, r.SelfLoop.PrimaryLAN.Addr)
	}
	return fmt.Sprintf("port :%d is LAN-reachable from self; external clients should be able to connect.", port)
}
