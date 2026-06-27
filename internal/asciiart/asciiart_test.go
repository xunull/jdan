package asciiart

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func solid(w, h int, c color.Color) *image.RGBA {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.Set(x, y, c)
		}
	}
	return m
}

func TestLuminance(t *testing.T) {
	if l := luminance(255, 255, 255); l != 255 {
		t.Errorf("white = %d, want 255", l)
	}
	if l := luminance(0, 0, 0); l != 0 {
		t.Errorf("black = %d, want 0", l)
	}
	if l := luminance(255, 0, 0); l < 70 || l > 80 {
		t.Errorf("red ≈ 76, got %d", l)
	}
}

func TestRender_SolidColors(t *testing.T) {
	// 全黑：亮度 0 → ramp[0] = 空格
	black := Render(solid(10, 10, color.Black), Options{Width: 5})
	for _, r := range black {
		if r != ' ' && r != '\n' {
			t.Errorf("全黑图应全是空格，出现了 %q", r)
		}
	}
	// 全白：亮度 255 → ramp 末尾 '@'
	white := Render(solid(10, 10, color.White), Options{Width: 5})
	if strings.Contains(strings.ReplaceAll(white, "\n", ""), " ") {
		t.Errorf("全白图不应有空格:\n%s", white)
	}
	if !strings.Contains(white, "@") {
		t.Errorf("全白图应是 '@':\n%s", white)
	}
}

func TestRender_Invert(t *testing.T) {
	// 全白 + invert → ramp[0] = 空格
	out := Render(solid(6, 6, color.White), Options{Width: 3, Invert: true})
	for _, r := range out {
		if r != ' ' && r != '\n' {
			t.Errorf("全白 + invert 应全空格，出现了 %q", r)
		}
	}
}

func TestRender_Color(t *testing.T) {
	red := solid(2, 2, color.RGBA{255, 0, 0, 255})
	withColor := Render(red, Options{Width: 2, Color: true})
	if !strings.Contains(withColor, "\x1b[38;2;255;0;0m") {
		t.Errorf("--color 应含红色 truecolor 码:\n%q", withColor)
	}
	noColor := Render(red, Options{Width: 2, Color: false})
	if strings.Contains(noColor, "\x1b") {
		t.Errorf("默认不应有 ANSI:\n%q", noColor)
	}
}

func TestRender_AspectRows(t *testing.T) {
	// 100×100，宽 80，charAspect 0.5 → 行数 = 80*1*0.5 = 40
	out := Render(solid(100, 100, color.Gray{128}), Options{Width: 80, CharAspect: 0.5})
	rows := strings.Count(strings.TrimRight(out, "\n"), "\n") + 1
	if rows != 40 {
		t.Errorf("行数 = %d, want 40（长宽比校正）", rows)
	}
}

func TestRender_WidthClampToImage(t *testing.T) {
	// 图只有 3px 宽，要 80 列 → 钳到 3，避免每格 <1px
	out := Render(solid(3, 3, color.White), Options{Width: 80})
	line := strings.SplitN(out, "\n", 2)[0]
	if len([]rune(line)) > 3 {
		t.Errorf("列宽应被钳到原图宽 3，得到 %d", len([]rune(line)))
	}
}

func TestResolveRamp(t *testing.T) {
	if ResolveRamp("") != RampStandard || ResolveRamp("standard") != RampStandard {
		t.Error("standard 解析错")
	}
	if ResolveRamp("detailed") != RampDetailed {
		t.Error("detailed 解析错")
	}
	if ResolveRamp("blocks") != RampBlocks {
		t.Error("blocks 解析错")
	}
	if ResolveRamp(" .#") != " .#" {
		t.Error("自定义 ramp 应原样")
	}
}
