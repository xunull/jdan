// Package asciiart 把图片渲染成 ASCII 字符画：按列宽切网格、块平均采样、亮度映射到
// 字符 ramp，可选每字符 truecolor 染色。纯逻辑，解码由调用方做（复用 stdlib image）。
package asciiart

import (
	"fmt"
	"image"
	"strings"
)

// 预设 ramp（暗 → 亮）。Blocks 用 East-Asian-ambiguous 方块，CJK 终端会变 2 列宽，
// 由 CLI 层警告。
const (
	RampStandard = " .:-=+*#%@"
	RampDetailed = " .'`^\",:;Il!i><~+_-?][}{1)(|\\/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$"
	RampBlocks   = " ░▒▓█"
)

// Options 控制渲染。
type Options struct {
	Width      int     // 目标列宽（0 → 80）；会被原图宽度上限钳制，避免每格 <1px
	Ramp       string  // 字符 ramp（暗→亮）；空 → RampStandard
	Invert     bool    // 反明暗（浅底终端 / 打印用）
	Color      bool    // 每字符按源像素 24-bit truecolor 染色
	CharAspect float64 // 字符高/宽比，0 → 0.5（终端字符约 2 倍高，纵向要压一半）
}

// Render 把已解码的图片渲染成字符画字符串。纯函数。
func Render(img image.Image, opts Options) string {
	b := img.Bounds()
	imgW, imgH := b.Dx(), b.Dy()
	if imgW == 0 || imgH == 0 {
		return ""
	}

	cols := opts.Width
	if cols <= 0 {
		cols = 80
	}
	if cols > imgW {
		cols = imgW // 不放大超过原图，保证每格 ≥ 1px
	}
	aspect := opts.CharAspect
	if aspect == 0 {
		aspect = 0.5
	}
	rows := max(int(float64(cols)*float64(imgH)/float64(imgW)*aspect), 1)

	ramp := opts.Ramp
	if ramp == "" {
		ramp = RampStandard
	}
	runes := []rune(ramp)
	n := len(runes)

	var sb strings.Builder
	for ry := range rows {
		y0 := b.Min.Y + ry*imgH/rows
		y1 := b.Min.Y + (ry+1)*imgH/rows
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for rx := range cols {
			x0 := b.Min.X + rx*imgW/cols
			x1 := b.Min.X + (rx+1)*imgW/cols
			if x1 <= x0 {
				x1 = x0 + 1
			}
			r, g, bl := avgColor(img, x0, y0, x1, y1)
			lum := luminance(r, g, bl)
			idx := lum * (n - 1) / 255
			if opts.Invert {
				idx = (255 - lum) * (n - 1) / 255
			}
			if opts.Color {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", r, g, bl)
			}
			sb.WriteRune(runes[idx])
		}
		if opts.Color {
			sb.WriteString("\x1b[0m")
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// avgColor 返回某矩形区域的平均 R/G/B（各 0–255）。
func avgColor(img image.Image, x0, y0, x1, y1 int) (r, g, b int) {
	var sr, sg, sb, cnt int64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA() // 各 0–65535（alpha 预乘）
			sr += int64(cr >> 8)
			sg += int64(cg >> 8)
			sb += int64(cb >> 8)
			cnt++
		}
	}
	if cnt == 0 {
		return 0, 0, 0
	}
	return int(sr / cnt), int(sg / cnt), int(sb / cnt)
}

// luminance 用 Rec.601 加权（0–255）。
func luminance(r, g, b int) int {
	return (r*299 + g*587 + b*114) / 1000
}

// ResolveRamp 把关键字（standard/detailed/blocks）解析成 ramp，其它当字面 ramp。
func ResolveRamp(s string) string {
	switch s {
	case "", "standard":
		return RampStandard
	case "detailed":
		return RampDetailed
	case "blocks":
		return RampBlocks
	default:
		return s
	}
}
