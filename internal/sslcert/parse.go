package sslcert

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// ParsePEMFile 从一个文件读 PEM 编码的证书。文件可以包含多个 cert（chain），
// 顺序假定 leaf 第一。
//
// 支持 "CERTIFICATE" PEM block；其他 block type（PRIVATE KEY 等）会被静默跳过。
func ParsePEMFile(path string) (*Bundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParsePEMBytes(data, "file:"+path)
}

// ParsePEMBytes 从 in-memory PEM 字节解析。
func ParsePEMBytes(data []byte, source string) (*Bundle, error) {
	var chain []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse certificate: %w", err)
		}
		chain = append(chain, c)
	}
	if len(chain) == 0 {
		return nil, errors.New("no CERTIFICATE PEM block found")
	}
	return &Bundle{
		Source: source,
		Chain:  chain,
	}, nil
}

// EncodePEM 把 chain 的所有 cert 编成标准 PEM 字符串（给 --pem 输出用）。
func EncodePEM(chain []*x509.Certificate) string {
	var out []byte
	for _, c := range chain {
		out = append(out, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: c.Raw,
		})...)
	}
	return string(out)
}
