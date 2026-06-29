// Package cdnx 识别一个站点前面挂的是哪家 CDN/WAF（Cloudflare 做得最深）。
//
// 三路互相独立的信号，任一命中即报，多路一致就定性"确定"：
//   - HTTP 响应头指纹（如 Cloudflare 的 CF-RAY、CloudFront 的 x-amz-cf-id）
//   - DNS NS 记录（域名是否托管在该 CDN 的 DNS，如 *.ns.cloudflare.com）
//   - 解析 IP 是否落在该 CDN 公布的网段（目前仅 Cloudflare 内嵌完整段）
//
// 本包是纯函数、不联网：网络采集（HTTP 拉头 / DNS 解析）由 CLI 负责后喂进来，
// 这样核心判定逻辑能脱网快测。0 新依赖（全 stdlib）。
package cdnx

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
)

// HeaderSig 是一条响应头指纹。Key 必须小写。
// Contains 非空时还要求头值（小写后）包含该子串；为空则只判键存在。
// Strong 标记"铁证级"指纹（如 cf-ray / x-amz-cf-id），单条命中即可定性"确定"。
type HeaderSig struct {
	Key      string
	Contains string
	Strong   bool
}

// Provider 描述一家 CDN/WAF 的识别特征。
type Provider struct {
	Name       string
	Headers    []HeaderSig
	NSSuffixes []string       // 小写、含前导点，如 ".ns.cloudflare.com"
	Ranges     []netip.Prefix // 公布的 IP 段（目前仅 Cloudflare 填满）
}

// Signal 是一条命中的证据。
type Signal struct {
	Kind   string `json:"kind"` // "header" | "ns" | "ip"
	Detail string `json:"detail"`
	Strong bool   `json:"strong,omitempty"`
}

// Match 是一家 provider 的命中结果。
type Match struct {
	Provider   string   `json:"provider"`
	Confidence string   `json:"confidence"` // "确定" | "很可能"
	Signals    []Signal `json:"signals"`
}

// Result 是一次检测的完整结果。
type Result struct {
	Matches     []Match  `json:"matches"`
	Colo        string   `json:"colo,omitempty"` // Cloudflare CF-RAY 里的边缘机房码（IATA 机场码）
	FinalURL    string   `json:"final_url,omitempty"`
	Host        string   `json:"host,omitempty"`
	IPs         []string `json:"ips,omitempty"`
	NS          []string `json:"ns,omitempty"`
	SyntheticIP bool     `json:"synthetic_ip,omitempty"` // 解析到的 IP 全是内网/fake-ip，段判定无意义
}

// benchPrefix 是 RFC2544 benchmarking 段，Clash/Surge 等 fake-ip 代理默认拿它当合成 IP。
var benchPrefix = netip.MustParsePrefix("198.18.0.0/15")

// looksSynthetic 判断 IP 是否像本地代理 fake-ip / 内网地址——这类 IP 做"落段"判定没意义。
func looksSynthetic(a netip.Addr) bool {
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
		a.IsUnspecified() || benchPrefix.Contains(a)
}

// Detected 是否命中任何 CDN。
func (r Result) Detected() bool { return len(r.Matches) > 0 }

// Detect 拿已采集的（小写键）响应头 / NS 记录 / 解析 IP，对照 providers 给出命中。纯函数。
func Detect(headers map[string]string, ns []string, ips []netip.Addr, providers []Provider) Result {
	var res Result
	for _, p := range providers {
		var sigs []Signal
		strong := false
		kinds := map[string]bool{}

		for _, h := range p.Headers {
			v, ok := headers[h.Key]
			if !ok {
				continue
			}
			if h.Contains != "" && !strings.Contains(strings.ToLower(v), h.Contains) {
				continue
			}
			detail := h.Key
			if v != "" {
				detail = h.Key + ": " + v
			}
			sigs = append(sigs, Signal{Kind: "header", Detail: detail, Strong: h.Strong})
			kinds["header"] = true
			if h.Strong {
				strong = true
			}
		}

		for _, n := range ns {
			nl := strings.ToLower(strings.TrimSuffix(n, "."))
			for _, suf := range p.NSSuffixes {
				if strings.HasSuffix(nl, suf) {
					sigs = append(sigs, Signal{Kind: "ns", Detail: "NS " + nl})
					kinds["ns"] = true
					break
				}
			}
		}

		for _, ip := range ips {
			for _, rng := range p.Ranges {
				if rng.Contains(ip) {
					sigs = append(sigs, Signal{Kind: "ip", Detail: ip.String() + " ∈ " + rng.String(), Strong: true})
					kinds["ip"] = true
					strong = true
					break
				}
			}
		}

		if len(sigs) == 0 {
			continue
		}
		conf := "很可能"
		if strong || len(kinds) >= 2 {
			conf = "确定"
		}
		res.Matches = append(res.Matches, Match{Provider: p.Name, Confidence: conf, Signals: sigs})
	}

	if v, ok := headers["cf-ray"]; ok {
		res.Colo = ColoFromCFRay(v)
	}

	// 全部解析 IP 都是合成/内网 → 标记，提示用户"落段"这一路在本机环境无效
	if len(ips) > 0 {
		res.SyntheticIP = true
		for _, ip := range ips {
			if !looksSynthetic(ip) {
				res.SyntheticIP = false
				break
			}
		}
	}
	return res
}

// ColoFromCFRay 从 CF-RAY（形如 "8a1f2c3d4e5f6789-SJC"）取末尾的边缘机房 IATA 码。
// 取不到合法 3+ 字母码时返回 ""。
func ColoFromCFRay(ray string) string {
	ray = strings.TrimSpace(ray)
	i := strings.LastIndex(ray, "-")
	if i < 0 {
		return ""
	}
	colo := strings.TrimSpace(ray[i+1:])
	if len(colo) < 3 {
		return ""
	}
	for _, c := range colo {
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return ""
		}
	}
	return strings.ToUpper(colo)
}

// Render 把结果渲染成人读文本。
func Render(r Result) string {
	var b strings.Builder
	if r.Detected() {
		tags := make([]string, len(r.Matches))
		for i, m := range r.Matches {
			tags[i] = fmt.Sprintf("%s（%s）", m.Provider, m.Confidence)
		}
		fmt.Fprintf(&b, "✅ %s\n", strings.Join(tags, " + "))
		if r.Colo != "" {
			fmt.Fprintf(&b, "   经 %s 边缘\n", r.Colo)
		}
	} else {
		b.WriteString("❌ 没看到已知 CDN\n")
	}
	if r.FinalURL != "" {
		fmt.Fprintf(&b, "   最终 URL：%s\n", r.FinalURL)
	}

	for _, m := range r.Matches {
		fmt.Fprintf(&b, "\n%s：\n", m.Provider)
		for _, s := range m.Signals {
			star := ""
			if s.Strong {
				star = " ★"
			}
			fmt.Fprintf(&b, "   · [%s] %s%s\n", s.Kind, s.Detail, star)
		}
	}

	// not-detected 时把采集到的原始 NS/IP 摆出来，方便人工判断
	if !r.Detected() {
		if len(r.NS) > 0 {
			fmt.Fprintf(&b, "   NS：%s\n", strings.Join(r.NS, ", "))
		}
		if len(r.IPs) > 0 {
			fmt.Fprintf(&b, "   IP：%s\n", strings.Join(r.IPs, ", "))
		}
	}
	if r.SyntheticIP {
		b.WriteString("   ⚠ 解析 IP 像本地代理 fake-ip / 内网，IP 段判定已失效；结论只基于响应头与 NS\n")
	}
	return b.String()
}

// FormatJSON 渲染成机读 JSON，顶层带一个 detected 布尔便于脚本判断。
func FormatJSON(r Result) (string, error) {
	data, err := json.MarshalIndent(struct {
		Detected bool `json:"detected"`
		Result
	}{Detected: r.Detected(), Result: r}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
