// Package sslcert 实现 jdan ssl cert 子命令的核心：
// 取一个 host 的 TLS 证书 chain，或从本地 PEM 文件解析，描述每一级
// cert 的关键字段，做 trust+hostname+expiry 三项验证，再可选查 OCSP。
//
// 设计意图：把"看证书"从 openssl s_client + openssl x509 -text -noout
// 这一坨调用 + 解析的痛苦，压成 jdan ssl cert <host> 一行命令 + 漂亮输出。
// 复用 internal/netprobe 的 TLS 握手逻辑——但保持独立 package，避免 ssl
// 命令依赖 netprobe 的整套阶段编排。
package sslcert

import (
	"crypto/x509"
	"time"
)

// Bundle 是一次取回的证书"包"：
//   - Chain[0] 是 leaf cert
//   - Chain[1..N-1] 是 server 发出的 intermediate cert
//   - 通过 system trust store 验证时，root 不一定在 Chain 里（系统自带）
//   - Source 记录数据来源（"host:port" 或 "file:cert.pem"）
//   - VerifiedChains 是 Verify() 填充的，可能包含 root（来自 trust store）
type Bundle struct {
	Source         string
	Chain          []*x509.Certificate
	VerifiedChains [][]*x509.Certificate

	// Host 是 TLS 握手时用的 SNI / hostname；本地文件 source 时为空
	Host string
}

// Leaf 返回 Chain[0]（如果 chain 非空）。
func (b *Bundle) Leaf() *x509.Certificate {
	if len(b.Chain) == 0 {
		return nil
	}
	return b.Chain[0]
}

// RootFromTrust 返回从 VerifiedChains 找到的 root（self-signed）cert。
// 如果 Verify 没跑或没找到 → nil。
func (b *Bundle) RootFromTrust() *x509.Certificate {
	for _, chain := range b.VerifiedChains {
		if len(chain) == 0 {
			continue
		}
		last := chain[len(chain)-1]
		if last != nil && isSelfSigned(last) {
			return last
		}
	}
	return nil
}

func isSelfSigned(c *x509.Certificate) bool {
	if c == nil {
		return false
	}
	return c.Subject.String() == c.Issuer.String()
}

// FullChain 返回 server-provided chain + 来自 trust store 的 root（如果有）。
// 用于 Render 时显示完整三层（leaf / intermediate / root）。
func (b *Bundle) FullChain() []*x509.Certificate {
	out := append([]*x509.Certificate{}, b.Chain...)
	if root := b.RootFromTrust(); root != nil {
		// 避免重复加（如果 server 已经发了 root）
		alreadyIncluded := false
		for _, c := range out {
			if c != nil && c.Equal(root) {
				alreadyIncluded = true
				break
			}
		}
		if !alreadyIncluded {
			out = append(out, root)
		}
	}
	return out
}

// Now 是单测可替换的"当前时间"，让 expires-in 测试不依赖真实时钟。
var Now = func() time.Time { return time.Now() }
