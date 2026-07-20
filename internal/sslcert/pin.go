package sslcert

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
)

// SPKI hashing for cert pinning.
//
// ── 重要：SPKI hash 不等于 cert fingerprint ────────────────────────────
//
//   Certificate fingerprint = SHA256(cert.Raw)               ← Describe() 里的字段
//   SPKI hash               = SHA256(cert.RawSubjectPublicKeyInfo)
//
// cert pinning（HPKP / OkHttp / iOS NSAppTransportSecurity /
// Android Network Security Config）**统一用 SPKI hash**：
//   - cert 经常 renew（同 key），renew 后 cert fingerprint 变了，
//     pinning 就坏；SPKI hash 在 key 不变时 stable
//   - HPKP RFC 7469 / Chrome static pins / iOS Apple Doc 都明确要 SPKI

// SPKIHash 返回 SHA256(cert.RawSubjectPublicKeyInfo) 的原始字节。
func SPKIHash(cert *x509.Certificate) []byte {
	if cert == nil {
		return nil
	}
	h := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return h[:]
}

// SPKIHashBase64 返回 SPKI hash 的 base64 standard encoding（带 `=` padding）。
// **不用 base64url 变体**——HPKP / OkHttp / iOS 都要 standard base64。
func SPKIHashBase64(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(SPKIHash(cert))
}

// PinEntry 描述一个 cert 的 pinning 信息（给 cli render 用）。
type PinEntry struct {
	Role       string `json:"role"` // "leaf" / "intermediate" / "root"
	SubjectCN  string `json:"subject_cn"`
	IssuerCN   string `json:"issuer_cn"`
	SPKISha256 string `json:"spki_sha256"` // base64 standard encoding
}

// EntryFromCert 从一个 cert 构造 PinEntry（不填 Role，由 caller 根据 chain 位置填）。
func EntryFromCert(cert *x509.Certificate) PinEntry {
	if cert == nil {
		return PinEntry{}
	}
	return PinEntry{
		SubjectCN:  cert.Subject.CommonName,
		IssuerCN:   cert.Issuer.CommonName,
		SPKISha256: SPKIHashBase64(cert),
	}
}

// ── 6 个 pin 格式器 ──────────────────────────────────────────────────────
//
// 每个返回适合 copy-paste 到对应配置文件 / 代码的字符串。
// host 在 raw / curl 场景下不使用，但保留签名一致让 cli render 统一调用。

// FormatOkHttp 输出 OkHttp (Android) CertificatePinner.Builder 链式调用。
//
//	CertificatePinner.Builder()
//	  .add("github.com", "sha256/E7gXi5lp1A0lRzgdv+0VG3UNQ4qfqLUjV3+oCYJqQVE=")
//	  .add("github.com", "sha256/x4QzPSC810K5/cMjb05Qm4k3Bw5zBn4lTdO/nEW/Td4=")
//	  .build()
func FormatOkHttp(host string, entries []PinEntry) string {
	if host == "" {
		host = "your-host.com" // -f 文件场景没 host，给 placeholder
	}
	var b strings.Builder
	b.WriteString("CertificatePinner.Builder()\n")
	for _, e := range entries {
		fmt.Fprintf(&b, "  .add(%q, \"sha256/%s\")\n", host, e.SPKISha256)
	}
	b.WriteString("  .build()")
	return b.String()
}

// FormatHPKP 输出 HTTP Public Key Pinning header（RFC 7469）。
// HPKP 已被浏览器弃用但内网 / 自有 client 场景仍在用。
//
//	Public-Key-Pins: pin-sha256="E7gXi5lp1A0lRzgdv+0VG3UNQ4qfqLUjV3+oCYJqQVE=";
//	                 pin-sha256="x4QzPSC810K5/cMjb05Qm4k3Bw5zBn4lTdO/nEW/Td4=";
//	                 max-age=5184000; includeSubDomains
func FormatHPKP(host string, entries []PinEntry) string {
	var b strings.Builder
	b.WriteString("Public-Key-Pins:")
	for _, e := range entries {
		fmt.Fprintf(&b, ` pin-sha256="%s";`, e.SPKISha256)
	}
	b.WriteString(" max-age=5184000; includeSubDomains")
	return b.String()
}

// FormatIOS 输出 iOS NSAppTransportSecurity NSPinnedDomains plist 片段。
// 注意：plist key 是 NSPinnedLeafIdentities 不是 NSPinnedCAIdentities（前者是 leaf；
// 后者是 CA / intermediate）——但 Apple 的命名约定一向把 SPKI 也称为 CA identity。
// 这里两个 key 都出。
func FormatIOS(host string, entries []PinEntry) string {
	if host == "" {
		host = "your-host.com"
	}
	var b strings.Builder
	b.WriteString("<key>NSAppTransportSecurity</key>\n")
	b.WriteString("<dict>\n")
	b.WriteString("  <key>NSPinnedDomains</key>\n")
	b.WriteString("  <dict>\n")
	fmt.Fprintf(&b, "    <key>%s</key>\n", host)
	b.WriteString("    <dict>\n")
	b.WriteString("      <key>NSIncludesSubdomains</key>\n")
	b.WriteString("      <true/>\n")
	b.WriteString("      <key>NSPinnedCAIdentities</key>\n")
	b.WriteString("      <array>\n")
	for _, e := range entries {
		b.WriteString("        <dict>\n")
		b.WriteString("          <key>SPKI-SHA256-BASE64</key>\n")
		fmt.Fprintf(&b, "          <string>%s</string>\n", e.SPKISha256)
		b.WriteString("        </dict>\n")
	}
	b.WriteString("      </array>\n")
	b.WriteString("    </dict>\n")
	b.WriteString("  </dict>\n")
	b.WriteString("</dict>")
	return b.String()
}

// FormatNSS 输出 Mozilla NSS / certutil 兼容格式。
//
//	pin-sha256:E7gXi5lp1A0lRzgdv+0VG3UNQ4qfqLUjV3+oCYJqQVE=
//	pin-sha256:x4QzPSC810K5/cMjb05Qm4k3Bw5zBn4lTdO/nEW/Td4=
func FormatNSS(host string, entries []PinEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "pin-sha256:%s", e.SPKISha256)
	}
	return b.String()
}

// FormatCurl 输出 curl --pinnedpubkey 兼容的 sha256// 格式。
// 多个 pin 用 `;` 分隔。
//
//	curl --pinnedpubkey 'sha256//E7gXi5lp1A0lRzgdv+0VG3UNQ4qfqLUjV3+oCYJqQVE=;sha256//x4QzPSC810K5/cMjb05Qm4k3Bw5zBn4lTdO/nEW/Td4='
func FormatCurl(host string, entries []PinEntry) string {
	pins := make([]string, 0, len(entries))
	for _, e := range entries {
		pins = append(pins, "sha256//"+e.SPKISha256)
	}
	return "curl --pinnedpubkey '" + strings.Join(pins, ";") + "' https://" + nonEmptyOr(host, "your-host.com") + "/"
}

// FormatRaw 输出纯 base64 hash，一行一个，给手撸代码 / 脚本用。
func FormatRaw(host string, entries []PinEntry) string {
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(e.SPKISha256)
	}
	return b.String()
}

func nonEmptyOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// PinFormatters 把所有格式器名 → 格式化函数 映射，给 cli 层 --format <name> 选用。
// "all" 不在 map 里，由 cli 层特判（默认输出所有 6 个）。
var PinFormatters = map[string]func(host string, entries []PinEntry) string{
	"okhttp": FormatOkHttp,
	"hpkp":   FormatHPKP,
	"ios":    FormatIOS,
	"nss":    FormatNSS,
	"curl":   FormatCurl,
	"raw":    FormatRaw,
}

// PinFormatNames 是按推荐展示顺序的格式名列表。
var PinFormatNames = []string{"okhttp", "ios", "hpkp", "nss", "curl", "raw"}
