package sslscan

import (
	"crypto/tls"
	"fmt"
)

// GradeReport 是综合评分输出。
type GradeReport struct {
	Score     int      `json:"score"`     // 0-100
	Letter    string   `json:"letter"`    // A+ / A / B / C / D / F
	Strengths []string `json:"strengths"` // 加分点
	Concerns  []string `json:"concerns"`  // 减分点
	Subscores Subscore `json:"subscores"`
}

// Subscore 是各维度分数（便于 debug 评分）
type Subscore struct {
	Cert     int `json:"cert"`            // 25 max
	Protocol int `json:"protocol"`        // 30 max
	KeyEx    int `json:"key_exchange"`    // 25 max（forward secrecy）
	Cipher   int `json:"cipher_strength"` // 20 max
	Modifier int `json:"modifier"`        // HSTS/preload bonus
}

// computeGrade 综合 ScanReport 各 section 给出 A+/A/B/C/D/F 评分。
//
// 评分逻辑借鉴 SSL Labs：5 维度加权 + 关键修饰符。
// 不是密码学评估——只是把"已知 best practice" 编码成分数。
func computeGrade(r *ScanReport) GradeReport {
	g := GradeReport{}

	g.Subscores.Cert = gradeCert(r.Cert, &g)
	g.Subscores.Protocol = gradeProtocol(r.Versions, &g)
	g.Subscores.KeyEx = gradeKeyExchange(r.Ciphers, r.Versions, &g)
	g.Subscores.Cipher = gradeCipherStrength(r.Ciphers, &g)
	g.Subscores.Modifier = gradeModifiers(r, &g)

	g.Score = g.Subscores.Cert + g.Subscores.Protocol + g.Subscores.KeyEx + g.Subscores.Cipher + g.Subscores.Modifier
	if g.Score > 100 {
		g.Score = 100
	}
	if g.Score < 0 {
		g.Score = 0
	}
	g.Letter = scoreToLetter(g.Score)
	return g
}

func gradeCert(c *CertSection, g *GradeReport) int {
	if c == nil {
		g.Concerns = append(g.Concerns, "could not retrieve certificate")
		return 0
	}
	score := 25
	if !c.Trusted {
		score -= 15
		g.Concerns = append(g.Concerns, "certificate chain not trusted by system store")
	}
	if !c.HostnameOK {
		score -= 10
		g.Concerns = append(g.Concerns, "certificate hostname does not match")
	}
	if c.Expired {
		score -= 20
		g.Concerns = append(g.Concerns, "certificate is expired")
	}
	if c.DaysLeft >= 0 && c.DaysLeft < 14 {
		score -= 5
		g.Concerns = append(g.Concerns, fmt.Sprintf("cert expires soon (%d days left)", c.DaysLeft))
	}
	if c.IsWeakSig {
		score -= 10
		g.Concerns = append(g.Concerns, "signature uses weak algorithm (SHA1/MD5)")
	}
	if c.IsWeakKey {
		score -= 10
		g.Concerns = append(g.Concerns, fmt.Sprintf("weak key size (%d bits)", c.KeySizeBits))
	}
	if score < 0 {
		score = 0
	}
	if score >= 20 {
		g.Strengths = append(g.Strengths, "certificate trusted and valid")
	}
	return score
}

func gradeProtocol(v VersionsSection, g *GradeReport) int {
	supported := v.SupportedSet()
	if len(supported) == 0 {
		g.Concerns = append(g.Concerns, "no TLS version supported")
		return 0
	}
	score := 30

	if supported[tls.VersionTLS13] {
		g.Strengths = append(g.Strengths, "TLS 1.3 supported")
	} else {
		score -= 10
		g.Concerns = append(g.Concerns, "TLS 1.3 not supported")
	}
	if !supported[tls.VersionTLS12] {
		score -= 10
		g.Concerns = append(g.Concerns, "TLS 1.2 not supported")
	}
	if supported[tls.VersionTLS11] {
		score -= 15
		g.Concerns = append(g.Concerns, "TLS 1.1 supported (deprecated)")
	}
	if supported[tls.VersionTLS10] {
		score -= 20
		g.Concerns = append(g.Concerns, "TLS 1.0 supported (deprecated)")
	}
	if score < 0 {
		score = 0
	}
	return score
}

func gradeKeyExchange(c CiphersSection, v VersionsSection, g *GradeReport) int {
	// TLS 1.3 强制 forward secrecy，所以 1.3 支持 = full points
	supported := v.SupportedSet()
	if supported[tls.VersionTLS13] {
		g.Strengths = append(g.Strengths, "TLS 1.3 enforces forward secrecy")
		return 25
	}

	// TLS 1.2 只走 cipher 看是否有 ECDHE / DHE
	if c.SupportedStrong() > 0 {
		return 25
	}
	if c.SupportedNonFS() > 0 {
		g.Concerns = append(g.Concerns, "non-forward-secret ciphers accepted")
		return 10
	}
	return 0
}

func gradeCipherStrength(c CiphersSection, g *GradeReport) int {
	score := 20
	if c.SupportedWeak() > 0 {
		score -= 15
		g.Concerns = append(g.Concerns, fmt.Sprintf("%d weak cipher(s) accepted (RC4/DES/3DES)", c.SupportedWeak()))
	}
	if c.SupportedStrong() > 0 {
		g.Strengths = append(g.Strengths, fmt.Sprintf("%d modern cipher(s) supported (AES-GCM/ChaCha20)", c.SupportedStrong()))
	}
	if score < 0 {
		score = 0
	}
	return score
}

func gradeModifiers(r *ScanReport, g *GradeReport) int {
	score := 0
	if r.HSTS != nil {
		switch r.HSTS.Strength() {
		case "preload":
			score += 5
			g.Strengths = append(g.Strengths, "HSTS with preload (1y + subdomains + preload list)")
		case "good":
			score += 3
			g.Strengths = append(g.Strengths, "HSTS configured (>= 1 year)")
		case "weak":
			score += 1
			g.Concerns = append(g.Concerns, "HSTS max-age < 1 year (weaker than recommended)")
		case "none":
			g.Concerns = append(g.Concerns, "HSTS not configured")
		}
	}
	if r.ALPN.HasH2() {
		score += 2
		g.Strengths = append(g.Strengths, "HTTP/2 supported via ALPN")
	}
	if r.Resume.AnySupported() {
		score += 1
		g.Strengths = append(g.Strengths, "session resumption supported (saves RTT on reconnect)")
	}
	return score
}

func scoreToLetter(score int) string {
	switch {
	case score >= 90:
		return "A+"
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 50:
		return "C"
	case score >= 35:
		return "D"
	}
	return "F"
}

// IsFailing 给 cli --grade-only flag 用：C 以下 exit 1。
func (g GradeReport) IsFailing() bool {
	return g.Score < 65 // C, D, F
}
