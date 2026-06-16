// Package imageinfo 实现 jdan img 命令的核心：只读图片文件头报出尺寸/格式/
// 颜色模型/大小，不解码整张图（用 image.DecodeConfig，快且省内存）。
//
// 支持 PNG / JPEG / GIF（注册 stdlib 解码器）。WEBP/BMP/TIFF 需要外部依赖
// golang.org/x/image，第一版不做——坚持 0 新依赖。
package imageinfo

import (
	"fmt"
	"image"
	"image/color"
	"io"

	// 注册 stdlib 解码器（DecodeConfig 用）。
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// Info 是一张图片的元信息。
type Info struct {
	Path   string `json:"path"`
	Format string `json:"format"` // png / jpeg / gif
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Color  string `json:"color"` // 颜色模型名：NRGBA / YCbCr / Gray / Paletted ...
	Bytes  int64  `json:"bytes"`
}

// HasAlpha 报告颜色模型是否含 alpha 通道。
func (i Info) HasAlpha() bool {
	switch i.Color {
	case "RGBA", "NRGBA", "RGBA64", "NRGBA64", "Alpha", "Alpha16":
		return true
	default:
		return false
	}
}

// Inspect 只读 header 拿图片配置。size 是文件/输入的字节数（调用方提供）。
func Inspect(path string, r io.Reader, size int64) (Info, error) {
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		return Info{}, fmt.Errorf("%s: %w", path, err)
	}
	return Info{
		Path:   path,
		Format: format,
		Width:  cfg.Width,
		Height: cfg.Height,
		Color:  ColorModelName(cfg.ColorModel),
		Bytes:  size,
	}, nil
}

// ColorModelName 把 color.Model 映射成可读名字。
func ColorModelName(m color.Model) string {
	switch m {
	case color.RGBAModel:
		return "RGBA"
	case color.NRGBAModel:
		return "NRGBA"
	case color.RGBA64Model:
		return "RGBA64"
	case color.NRGBA64Model:
		return "NRGBA64"
	case color.GrayModel:
		return "Gray"
	case color.Gray16Model:
		return "Gray16"
	case color.CMYKModel:
		return "CMYK"
	case color.AlphaModel:
		return "Alpha"
	case color.Alpha16Model:
		return "Alpha16"
	case color.YCbCrModel:
		return "YCbCr"
	case color.NYCbCrAModel:
		return "NYCbCrA"
	}
	// Paletted 图（GIF）的 ColorModel 是 color.Palette（非上述常量之一）。
	if _, ok := m.(color.Palette); ok {
		return "Paletted"
	}
	return "Unknown"
}

// HumanizeBytes 把字节数转成可读字符串（IEC 单位：B/KiB/MiB/GiB）。
func HumanizeBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
