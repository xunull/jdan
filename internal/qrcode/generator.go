// Package qrcode 生成二维码并把它渲染为终端字符串、PNG 或 SVG。
//
// 设计目标：不暴露底层 QR 库的细节，给上层 CLI 一个稳定的 Render*/Describe API；
// 其他子命令（比如计划中的 jdan http serve）可以直接复用 RenderTerminal 嵌入
// LAN URL 二维码。
package qrcode

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	gqr "github.com/skip2/go-qrcode"
)

// ECC 是 QR 容错等级。数字越大占用空间越大、抗损能力越强。
type ECC int

const (
	Low     ECC = iota // L: 容错 7%
	Medium             // M: 容错 15%
	High               // Q: 容错 25%
	Highest            // H: 容错 30%
)

func eccString(e ECC) string {
	switch e {
	case Low:
		return "L"
	case High:
		return "Q"
	case Highest:
		return "H"
	default:
		return "M"
	}
}

// ParseECC 解析 L/M/Q/H（大小写不敏感，允许首尾空格）。
func ParseECC(s string) (ECC, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "L":
		return Low, nil
	case "M":
		return Medium, nil
	case "Q":
		return High, nil
	case "H":
		return Highest, nil
	}
	return 0, fmt.Errorf("invalid ECC level %q (want L/M/Q/H)", s)
}

// Options 控制渲染细节。
type Options struct {
	ECC       ECC  // 容错等级
	Invert    bool // 反色（终端白底场景）
	FullBlock bool // 终端渲染时用全角 ██ 而不是默认半角 ▀▄
}

// DefaultOptions 返回合理默认值：M 级容错、不反色、半角块。
func DefaultOptions() Options {
	return Options{ECC: Medium}
}

func toLib(e ECC) gqr.RecoveryLevel {
	switch e {
	case Low:
		return gqr.Low
	case High:
		return gqr.High
	case Highest:
		return gqr.Highest
	default:
		return gqr.Medium
	}
}

// build 构造底层 QR 码并裁掉 quiet zone，返回方阵 bitmap（true = dark）。
// skip2/go-qrcode 的 Bitmap() 自带 4 modules 的 quiet zone，我们裁掉它然后由
// 各 Render* 决定要不要补回去（终端通常不补，因为终端背景就是 padding；
// SVG 单独补 quiet zone）。
func build(data string, opts Options) ([][]bool, error) {
	if data == "" {
		return nil, errors.New("empty data")
	}
	q, err := gqr.New(data, toLib(opts.ECC))
	if err != nil {
		return nil, err
	}
	bm := q.Bitmap()
	return stripQuietZone(bm, 4), nil
}

func stripQuietZone(b [][]bool, quiet int) [][]bool {
	n := len(b)
	if n <= quiet*2 {
		return b
	}
	out := make([][]bool, n-quiet*2)
	for i := range out {
		out[i] = make([]bool, n-quiet*2)
		copy(out[i], b[i+quiet][quiet:n-quiet])
	}
	return out
}

// RenderTerminal 输出可直接 Print 的字符串。半角模式下相邻两行像素压成一行
// （▀ 上 ▄ 下 █ 全 空格 无），高度减半。在二维码上下各加 1 行空白，给手机
// 摄像头一些 quiet zone。
func RenderTerminal(data string, opts Options) (string, error) {
	bitmap, err := build(data, opts)
	if err != nil {
		return "", err
	}
	if opts.FullBlock {
		return renderFull(bitmap, opts.Invert), nil
	}
	return renderHalf(bitmap, opts.Invert), nil
}

func renderHalf(b [][]bool, invert bool) string {
	n := len(b)
	var buf bytes.Buffer
	pad := strings.Repeat(" ", n)
	buf.WriteString(pad)
	buf.WriteByte('\n')
	for i := 0; i < n; i += 2 {
		for j := range n {
			top := b[i][j]
			var bot bool
			if i+1 < n {
				bot = b[i+1][j]
			}
			if invert {
				top, bot = !top, !bot
			}
			switch {
			case top && bot:
				buf.WriteRune('█')
			case top && !bot:
				buf.WriteRune('▀')
			case !top && bot:
				buf.WriteRune('▄')
			default:
				buf.WriteByte(' ')
			}
		}
		buf.WriteByte('\n')
	}
	buf.WriteString(pad)
	buf.WriteByte('\n')
	return buf.String()
}

func renderFull(b [][]bool, invert bool) string {
	n := len(b)
	var buf bytes.Buffer
	pad := strings.Repeat("  ", n)
	buf.WriteString(pad)
	buf.WriteByte('\n')
	for _, row := range b {
		for _, cell := range row {
			on := cell
			if invert {
				on = !on
			}
			if on {
				buf.WriteString("██")
			} else {
				buf.WriteString("  ")
			}
		}
		buf.WriteByte('\n')
	}
	buf.WriteString(pad)
	buf.WriteByte('\n')
	return buf.String()
}

// RenderPNG 输出 size 像素的正方形 PNG。size <= 0 视为默认 256。
// 注意：底层库会在 PNG 里自动加 4-module quiet zone，不用调用方操心。
func RenderPNG(data string, opts Options, size int) ([]byte, error) {
	if data == "" {
		return nil, errors.New("empty data")
	}
	if size <= 0 {
		size = 256
	}
	q, err := gqr.New(data, toLib(opts.ECC))
	if err != nil {
		return nil, err
	}
	return q.PNG(size)
}

// RenderSVG 输出 SVG 字符串。每模块占 modulePixels × modulePixels 像素；
// modulePixels <= 0 时取默认 8。自动加 4 modules 的 quiet zone。
func RenderSVG(data string, opts Options, modulePixels int) (string, error) {
	bitmap, err := build(data, opts)
	if err != nil {
		return "", err
	}
	if modulePixels <= 0 {
		modulePixels = 8
	}
	n := len(bitmap)
	quiet := 4
	side := (n + quiet*2) * modulePixels

	bg, fg := "#ffffff", "#000000"
	if opts.Invert {
		bg, fg = "#000000", "#ffffff"
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintf(&buf,
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges">`,
		side, side, side, side)
	fmt.Fprintf(&buf, `<rect width="%d" height="%d" fill="%s"/>`, side, side, bg)
	for i, row := range bitmap {
		for j, on := range row {
			if !on {
				continue
			}
			x := (j + quiet) * modulePixels
			y := (i + quiet) * modulePixels
			fmt.Fprintf(&buf, `<rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
				x, y, modulePixels, modulePixels, fg)
		}
	}
	buf.WriteString(`</svg>`)
	return buf.String(), nil
}

// Info 是 --json 输出的结构。Modules 是不含 quiet zone 的方阵边长。
type Info struct {
	Data    string `json:"data"`
	ECC     string `json:"ecc"`
	Modules int    `json:"modules"`
}

// Describe 返回 QR 码的元数据，不实际渲染。
func Describe(data string, opts Options) (Info, error) {
	bitmap, err := build(data, opts)
	if err != nil {
		return Info{}, err
	}
	return Info{
		Data:    data,
		ECC:     eccString(opts.ECC),
		Modules: len(bitmap),
	}, nil
}
