// Package extract 实现 jdan extract 命令：识别 archive 格式（按文件扩展名）
// 并解压到目标目录。
//
// 支持：.zip / .tar / .tar.gz / .tgz / .tar.bz2 / .tbz2 / .gz / .bz2
// 不支持：.7z（外部 lib 复杂）/ .tar.xz（dep 太重，v1 不做）
//
// 安全：tar / zip 都做 directory traversal 防护——entry 名含 `..` 或
// 解压后落到 root 外的，一律拒绝。这是文件解压工具最常见的 CVE 来源。
package extract

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Format 是被识别的 archive 格式。
type Format string

const (
	FormatZip    Format = "zip"
	FormatTar    Format = "tar"
	FormatTarGz  Format = "tar.gz"
	FormatTarBz2 Format = "tar.bz2"
	FormatGz     Format = "gz"  // 单文件 gzip
	FormatBz2    Format = "bz2" // 单文件 bzip2
)

// DetectFormat 按文件名后缀识别格式。返回 ErrUnknownFormat 让 cli 给出友好错误。
func DetectFormat(path string) (Format, error) {
	low := strings.ToLower(path)
	switch {
	case strings.HasSuffix(low, ".zip"):
		return FormatZip, nil
	case strings.HasSuffix(low, ".tar.gz"), strings.HasSuffix(low, ".tgz"):
		return FormatTarGz, nil
	case strings.HasSuffix(low, ".tar.bz2"), strings.HasSuffix(low, ".tbz2"), strings.HasSuffix(low, ".tbz"):
		return FormatTarBz2, nil
	case strings.HasSuffix(low, ".tar"):
		return FormatTar, nil
	case strings.HasSuffix(low, ".gz"):
		return FormatGz, nil
	case strings.HasSuffix(low, ".bz2"):
		return FormatBz2, nil
	}
	return "", fmt.Errorf("%w: %s (supported: .zip .tar .tar.gz .tgz .tar.bz2 .tbz2 .gz .bz2)",
		ErrUnknownFormat, filepath.Base(path))
}

// ErrUnknownFormat 表示文件扩展名不在 jdan extract 支持列表里。
var ErrUnknownFormat = errors.New("unknown archive format")

// Entry 是一个 archive 条目的元信息（给 --list 用，不实际解压时填充）。
type Entry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	Mode    string `json:"mode,omitempty"`
}

// Options 控制解压行为。
type Options struct {
	OutDir   string // 解压目标目录（绝对或相对）；必须已存在或可创建
	ListOnly bool   // 只列内容不写文件
}

// Extract 是主入口：识别格式 → 调用对应的 extractor。
//
// 返回：list 模式下 entries 是 archive 内容；非 list 模式下空（写到 OutDir）。
func Extract(archivePath string, opts Options) ([]Entry, error) {
	fmt_, err := DetectFormat(archivePath)
	if err != nil {
		return nil, err
	}

	// 确保 OutDir 存在（list 模式不创建）
	if !opts.ListOnly && opts.OutDir != "" {
		if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", opts.OutDir, err)
		}
	}

	switch fmt_ {
	case FormatZip:
		return extractZip(archivePath, opts)
	case FormatTar:
		return extractTar(archivePath, opts, nil)
	case FormatTarGz:
		return extractTarCompressed(archivePath, opts, openGzip)
	case FormatTarBz2:
		return extractTarCompressed(archivePath, opts, openBzip2)
	case FormatGz:
		return extractSingleCompressed(archivePath, opts, openGzip, ".gz")
	case FormatBz2:
		return extractSingleCompressed(archivePath, opts, openBzip2, ".bz2")
	}
	return nil, fmt.Errorf("unhandled format: %s", fmt_)
}

// ─── zip ────────────────────────────────────────────────────────────────

func extractZip(archivePath string, opts Options) ([]Entry, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var entries []Entry
	for _, f := range r.File {
		entries = append(entries, Entry{
			Name:  f.Name,
			Size:  int64(f.UncompressedSize64),
			IsDir: f.FileInfo().IsDir(),
			Mode:  f.Mode().String(),
		})
		if opts.ListOnly {
			continue
		}
		if err := writeZipEntry(f, opts.OutDir); err != nil {
			return nil, fmt.Errorf("extract %s: %w", f.Name, err)
		}
	}
	return entries, nil
}

func writeZipEntry(f *zip.File, outDir string) error {
	target, err := safeJoin(outDir, f.Name)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, f.Mode())
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm())
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = copyLimited(dst, src)
	return err
}

// ─── tar / tar.gz / tar.bz2 ────────────────────────────────────────────

type decompressOpen func(io.Reader) (io.Reader, io.Closer, error)

func openGzip(r io.Reader) (io.Reader, io.Closer, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, nil, err
	}
	return gz, gz, nil
}

func openBzip2(r io.Reader) (io.Reader, io.Closer, error) {
	// compress/bzip2 不返回 Closer，包一个 no-op
	return bzip2.NewReader(r), nopCloser{}, nil
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// extractTarCompressed 处理 .tar.gz / .tar.bz2：先解压外层，再过 tar reader。
func extractTarCompressed(archivePath string, opts Options, opener decompressOpen) ([]Entry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	inner, closer, err := opener(f)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return extractTar(archivePath, opts, inner)
}

// extractTar 实际处理 tar 流。reader 为 nil 时从 archivePath 读 raw tar。
func extractTar(archivePath string, opts Options, reader io.Reader) ([]Entry, error) {
	if reader == nil {
		f, err := os.Open(archivePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		reader = f
	}

	tr := tar.NewReader(reader)
	var entries []Entry
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar header: %w", err)
		}

		entry := Entry{
			Name:  h.Name,
			Size:  h.Size,
			IsDir: h.Typeflag == tar.TypeDir,
			Mode:  os.FileMode(h.Mode).String(),
		}
		entries = append(entries, entry)
		if opts.ListOnly {
			continue
		}
		if err := writeTarEntry(tr, h, opts.OutDir); err != nil {
			return nil, fmt.Errorf("extract %s: %w", h.Name, err)
		}
	}
	return entries, nil
}

func writeTarEntry(tr *tar.Reader, h *tar.Header, outDir string) error {
	target, err := safeJoin(outDir, h.Name)
	if err != nil {
		return err
	}
	mode := os.FileMode(h.Mode)
	switch h.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, mode|0o755)
	case tar.TypeReg:
		// 注：tar.TypeRegA 在新版 stdlib 已 deprecated，stdlib reader
		// 自动把它转成 TypeReg，所以不用单独处理。
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = copyLimited(f, tr)
		return err
	case tar.TypeSymlink:
		// 不解 symlink——避免 symlink 指向 root 外造成 escape
		// (老 Solaris pax 攻击：symlink to / then file 写入)
		return nil
	}
	// 其他类型（block/char device / fifo）跳过
	return nil
}

// ─── 单文件 gz / bz2 ───────────────────────────────────────────────────

// extractSingleCompressed 处理 file.gz / file.bz2：解压结果是单个文件，
// 命名为 file（去掉 .gz / .bz2 后缀）。
func extractSingleCompressed(archivePath string, opts Options, opener decompressOpen, suffix string) ([]Entry, error) {
	in, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer in.Close()
	dec, closer, err := opener(in)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	// 输出文件名：去掉后缀
	base := filepath.Base(archivePath)
	outName := strings.TrimSuffix(base, suffix)
	if outName == base {
		// 不带 .gz / .bz2 后缀（极少见），给个 .out 防覆盖原文件
		outName = base + ".out"
	}

	entry := Entry{Name: outName, IsDir: false}

	if opts.ListOnly {
		// list 模式没法 cheap 拿 size（要解压才知道），留 0
		return []Entry{entry}, nil
	}

	target, err := safeJoin(opts.OutDir, outName)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(target)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	n, err := copyLimited(f, dec)
	if err != nil {
		return nil, err
	}
	entry.Size = n
	return []Entry{entry}, nil
}

// ─── 安全：directory traversal 防护 ─────────────────────────────────────

// safeJoin 把 root + entryName join 成 absolute path，并验证结果仍在 root 下。
// 拒绝任何 `..` 跳出 root 的尝试 + 拒绝绝对路径 entry。
//
// 这是 archive 解压最常见的 CVE 来源（zip slip）。我们选 **reject** 而不是
// **silent sanitize**：用户应当知道这个 archive 是恶意的，而不是被默默改名。
func safeJoin(root, entryName string) (string, error) {
	// 1. 拒绝绝对路径 entry（"/etc/passwd" → reject）
	if filepath.IsAbs(entryName) {
		return "", fmt.Errorf("entry %q has absolute path", entryName)
	}
	// 2. clean entry name（不加 / 前缀，让 ".." 保留）
	clean := filepath.Clean(entryName)
	// 3. 拒绝任何 `..` 段（".." / "../foo" / "sub/../foo" 等）
	for _, seg := range strings.Split(clean, string(filepath.Separator)) {
		if seg == ".." {
			return "", fmt.Errorf("entry %q contains '..' (directory traversal)", entryName)
		}
	}
	// 4. join 到 root
	target := filepath.Join(root, clean)
	// 5. paranoid check: 即使前面通过了，再验证一次 absolute path 在 root 下
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(targetAbs+string(filepath.Separator), rootAbs+string(filepath.Separator)) &&
		targetAbs != rootAbs {
		return "", fmt.Errorf("entry %q escapes root %q", entryName, root)
	}
	return target, nil
}

// copyLimited 用 io.Copy 但带"防 zip bomb"上限。
// 4 GiB 单 entry 应付绝大多数合理 archive；恶意 zip bomb 通常解压几 TB。
//
// 必须显式标 int64：untyped 常量传给 fmt.Errorf 的 ...any 时会默认成 int，
// 在 32 位平台（linux/386、linux/arm）上 4294967296 溢出 int32，整个模块编译不过。
const maxEntrySize int64 = 4 << 30 // 4 GiB

func copyLimited(dst io.Writer, src io.Reader) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, maxEntrySize+1))
	if err != nil {
		return n, err
	}
	if n > maxEntrySize {
		return n, fmt.Errorf("entry exceeds %d bytes (refusing zip-bomb-shape)", maxEntrySize)
	}
	return n, nil
}

// DefaultOutDir 给 cli 用：决定默认解压目录。
// `.tar.gz` / `.tar.bz2` / `.tgz` 全用基本名（去掉双后缀）；其他用单后缀。
func DefaultOutDir(archivePath string) string {
	base := filepath.Base(archivePath)
	low := strings.ToLower(base)
	for _, suf := range []string{".tar.gz", ".tar.bz2", ".tbz2", ".tbz", ".tgz", ".zip", ".tar", ".gz", ".bz2"} {
		if strings.HasSuffix(low, suf) {
			return strings.TrimSuffix(base, base[len(base)-len(suf):])
		}
	}
	return base + "-extracted"
}
