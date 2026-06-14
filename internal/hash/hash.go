// Package hash 实现 jdan hash 命令的核心：streaming md5/sha1/sha256/sha512
// 多算法并行（一遍读取，io.MultiWriter feed 多个 hasher），加上 macOS
// shasum -c / Linux sha256sum -c 兼容的 --check 校验解析。
//
// 设计要点：
//   - 文件可能 1GB+，必须 streaming，不能全读进内存
//   - 多算法同时跑（--algo md5,sha256 一遍读取出两个 hash）
//   - 接受 stdin（- 路径）让 echo / cat 管道能用
//   - --check 校验跟系统工具的输出 byte-equal，能交叉验证
package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"strings"
)

// Algorithm 是被支持的哈希算法名。
type Algorithm string

const (
	AlgoMD5    Algorithm = "md5"
	AlgoSHA1   Algorithm = "sha1"
	AlgoSHA256 Algorithm = "sha256"
	AlgoSHA512 Algorithm = "sha512"
)

// AllAlgorithms 返回 `--all` 默认包含的算法列表。
func AllAlgorithms() []Algorithm {
	return []Algorithm{AlgoMD5, AlgoSHA1, AlgoSHA256, AlgoSHA512}
}

// ParseAlgorithms 把 "md5,sha256" 这样的 csv 解析成 Algorithm slice。
// 不区分大小写；空字符串返回错误；未知算法返回错误。
func ParseAlgorithms(s string) ([]Algorithm, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty algorithm list")
	}
	parts := strings.Split(s, ",")
	out := make([]Algorithm, 0, len(parts))
	seen := make(map[Algorithm]bool)
	for _, p := range parts {
		a, err := parseSingle(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		if !seen[a] {
			out = append(out, a)
			seen[a] = true
		}
	}
	return out, nil
}

func parseSingle(s string) (Algorithm, error) {
	switch strings.ToLower(s) {
	case "md5":
		return AlgoMD5, nil
	case "sha1":
		return AlgoSHA1, nil
	case "sha256":
		return AlgoSHA256, nil
	case "sha512":
		return AlgoSHA512, nil
	}
	return "", fmt.Errorf("unknown algorithm %q (want md5/sha1/sha256/sha512)", s)
}

// newHasher 给一个 Algorithm 返回对应的 stdlib hasher。
func newHasher(a Algorithm) (hash.Hash, error) {
	switch a {
	case AlgoMD5:
		return md5.New(), nil
	case AlgoSHA1:
		return sha1.New(), nil
	case AlgoSHA256:
		return sha256.New(), nil
	case AlgoSHA512:
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unknown algorithm %q", a)
}

// Result 是一次 hash 调用的产出。Path 给 cli 层显示用；多算法时 Sums 含多条。
type Result struct {
	Path string            `json:"path,omitempty"`
	Sums map[Algorithm]string `json:"sums"` // hex lowercase
}

// Sum 按算法名取 hex；missing 返回空串。
func (r *Result) Sum(a Algorithm) string {
	if r == nil || r.Sums == nil {
		return ""
	}
	return r.Sums[a]
}

// SortedAlgos 让 cli render 输出顺序稳定（md5 → sha1 → sha256 → sha512）。
func (r *Result) SortedAlgos() []Algorithm {
	if r == nil {
		return nil
	}
	algos := make([]Algorithm, 0, len(r.Sums))
	for a := range r.Sums {
		algos = append(algos, a)
	}
	sort.Slice(algos, func(i, j int) bool {
		return algoOrder(algos[i]) < algoOrder(algos[j])
	})
	return algos
}

func algoOrder(a Algorithm) int {
	switch a {
	case AlgoMD5:
		return 1
	case AlgoSHA1:
		return 2
	case AlgoSHA256:
		return 3
	case AlgoSHA512:
		return 4
	}
	return 999
}

// HashReader 对一个 reader 同时跑 N 种算法（io.MultiWriter 一遍读取）。
// 返回 Result.Sums 是 algo → hex lowercase 的 map。
//
// 关键：多算法时不再多读 N 次，一次 read 喂 N 个 hasher。
func HashReader(r io.Reader, algos []Algorithm) (*Result, error) {
	if len(algos) == 0 {
		return nil, errors.New("at least one algorithm required")
	}

	hashers := make(map[Algorithm]hash.Hash, len(algos))
	writers := make([]io.Writer, 0, len(algos))
	for _, a := range algos {
		h, err := newHasher(a)
		if err != nil {
			return nil, err
		}
		hashers[a] = h
		writers = append(writers, h)
	}

	mw := io.MultiWriter(writers...)
	if _, err := io.Copy(mw, r); err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	result := &Result{Sums: make(map[Algorithm]string, len(algos))}
	for a, h := range hashers {
		result.Sums[a] = hex.EncodeToString(h.Sum(nil))
	}
	return result, nil
}

// CheckEntry 是 --check 校验文件里的一行。
type CheckEntry struct {
	Path     string
	Expected string // hex lowercase
}

// CheckResult 是 --check 模式针对一个 entry 的结果。
type CheckResult struct {
	Entry  CheckEntry
	Got    string // 实际算出的 hex
	Status string // "OK" / "FAILED" / "MISSING" / "ERROR"
	Err    string
}

// ParseChecksumLine 解析 shasum / sha256sum 输出行：
//
//	abc123def456...  filename
//	abc123def456... *filename     ← `*` 表示 binary mode
//
// 跳过空行 / 注释（# 开头）。无法解析返回 err。
func ParseChecksumLine(line string) (*CheckEntry, error) {
	line = strings.TrimRight(line, "\r\n")
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil, nil // skip
	}

	// macOS shasum / Linux sha256sum 用两空格分隔 hash 和 filename
	// (or one space + `*` for binary mode)
	idx := strings.Index(line, "  ")
	star := strings.Index(line, " *")
	switch {
	case idx >= 0 && (star < 0 || idx < star):
		hash := line[:idx]
		path := line[idx+2:]
		return &CheckEntry{Path: path, Expected: strings.ToLower(strings.TrimSpace(hash))}, nil
	case star >= 0:
		hash := line[:star]
		path := line[star+2:]
		return &CheckEntry{Path: path, Expected: strings.ToLower(strings.TrimSpace(hash))}, nil
	}
	return nil, fmt.Errorf("malformed checksum line: %q", line)
}

// ParseChecksumFile 把一个 checksum 文件全解析。无效行返回错误（而不是跳过），
// 因为校验意图是 "all-or-nothing"。
func ParseChecksumFile(r io.Reader) ([]CheckEntry, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var entries []CheckEntry
	for i, line := range strings.Split(string(data), "\n") {
		e, perr := ParseChecksumLine(line)
		if perr != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, perr)
		}
		if e != nil {
			entries = append(entries, *e)
		}
	}
	return entries, nil
}

// AlgoFromHexLength 根据 hex 长度推断算法（用于 --check 时自动选）：
//
//	32 chars = md5
//	40 chars = sha1
//	64 chars = sha256
//	128 chars = sha512
func AlgoFromHexLength(hexStr string) (Algorithm, error) {
	switch len(hexStr) {
	case 32:
		return AlgoMD5, nil
	case 40:
		return AlgoSHA1, nil
	case 64:
		return AlgoSHA256, nil
	case 128:
		return AlgoSHA512, nil
	}
	return "", fmt.Errorf("ambiguous hash length %d", len(hexStr))
}
