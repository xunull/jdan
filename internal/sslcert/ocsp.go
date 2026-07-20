package sslcert

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/ocsp"
)

// OCSPStatus 是单个 cert 的 OCSP 查询结果。
type OCSPStatus struct {
	Available    bool      `json:"available"` // cert 是否有 OCSP responder URL
	Checked      bool      `json:"checked"`   // 我们真的查了（没 skip）
	Status       string    `json:"status"`    // good / revoked / unknown
	Revoked      bool      `json:"revoked"`
	RevokedAt    time.Time `json:"revoked_at,omitempty"`
	Reason       string    `json:"revocation_reason,omitempty"` // 文字化
	ResponderURL string    `json:"responder_url,omitempty"`
	Err          string    `json:"error,omitempty"`
}

// OCSPHTTPClient 是可测试替换的 OCSP 请求 client。生产用默认 net/http，
// 单测可以注入返回 mock OCSP DER 字节。
var OCSPHTTPClient = &http.Client{Timeout: 3 * time.Second}

// CheckOCSP 查 cert 在 issuer 上的吊销状态。
//   - cert 没有 OCSP responder URL → Available=false, Checked=false
//   - 网络/parse 失败 → Checked=false + Err
//   - 拿到响应 → Status: good/revoked/unknown，Revoked 字段同步
func CheckOCSP(ctx context.Context, cert, issuer *x509.Certificate) OCSPStatus {
	s := OCSPStatus{}
	if cert == nil || issuer == nil {
		s.Err = "missing cert or issuer"
		return s
	}
	if len(cert.OCSPServer) == 0 {
		// 大多数 root cert 没 OCSP server，这是正常情况
		return s
	}
	s.Available = true
	s.ResponderURL = cert.OCSPServer[0]

	reqBytes, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		s.Err = fmt.Sprintf("build OCSP request: %v", err)
		return s
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", s.ResponderURL, bytes.NewReader(reqBytes))
	if err != nil {
		s.Err = fmt.Sprintf("build HTTP request: %v", err)
		return s
	}
	httpReq.Header.Set("Content-Type", "application/ocsp-request")
	httpReq.Header.Set("Accept", "application/ocsp-response")

	resp, err := OCSPHTTPClient.Do(httpReq)
	if err != nil {
		s.Err = fmt.Sprintf("OCSP request: %v", err)
		return s
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
	if err != nil {
		s.Err = fmt.Sprintf("read OCSP response: %v", err)
		return s
	}
	if resp.StatusCode != http.StatusOK {
		s.Err = fmt.Sprintf("OCSP responder HTTP %d", resp.StatusCode)
		return s
	}

	ocspResp, err := ocsp.ParseResponseForCert(respBytes, cert, issuer)
	if err != nil {
		s.Err = fmt.Sprintf("parse OCSP response: %v", err)
		return s
	}

	s.Checked = true
	switch ocspResp.Status {
	case ocsp.Good:
		s.Status = "good"
	case ocsp.Revoked:
		s.Status = "revoked"
		s.Revoked = true
		s.RevokedAt = ocspResp.RevokedAt
		s.Reason = revocationReasonString(ocspResp.RevocationReason)
	case ocsp.Unknown:
		s.Status = "unknown"
	default:
		s.Status = fmt.Sprintf("status=%d", ocspResp.Status)
	}
	return s
}

func revocationReasonString(r int) string {
	switch r {
	case ocsp.Unspecified:
		return "unspecified"
	case ocsp.KeyCompromise:
		return "keyCompromise"
	case ocsp.CACompromise:
		return "CACompromise"
	case ocsp.AffiliationChanged:
		return "affiliationChanged"
	case ocsp.Superseded:
		return "superseded"
	case ocsp.CessationOfOperation:
		return "cessationOfOperation"
	case ocsp.CertificateHold:
		return "certificateHold"
	case ocsp.RemoveFromCRL:
		return "removeFromCRL"
	case ocsp.PrivilegeWithdrawn:
		return "privilegeWithdrawn"
	case ocsp.AACompromise:
		return "AACompromise"
	}
	return fmt.Sprintf("reason(%d)", r)
}

// CheckChainOCSP 对 chain 里每个非 root cert 查 OCSP。返回 status 数组对齐 chain（包括 root 一项空 status）。
// chain[i] 的 issuer 是 chain[i+1]；root 没有 OCSP（self-signed）。
func CheckChainOCSP(ctx context.Context, chain []*x509.Certificate) []OCSPStatus {
	out := make([]OCSPStatus, len(chain))
	for i, c := range chain {
		if i+1 >= len(chain) {
			// 最后一级（root 或孤立 leaf）没 issuer 可对，skip
			continue
		}
		issuer := chain[i+1]
		// 用一个 short timeout 避免拖太久
		c2, cancel := context.WithTimeout(ctx, 3*time.Second)
		out[i] = CheckOCSP(c2, c, issuer)
		cancel()
	}
	return out
}
