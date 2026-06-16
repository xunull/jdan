package imageinfo

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

// pngBytes / jpegBytes / gifBytes 现场编码小图供测试。

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			img.Set(x, y, color.NRGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gifBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	pal := image.NewPaletted(image.Rect(0, 0, w, h), []color.Color{color.Black, color.White})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, pal, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInspectPNG(t *testing.T) {
	data := pngBytes(t, 40, 20)
	info, err := Inspect("t.png", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "png" {
		t.Errorf("format = %q, want png", info.Format)
	}
	if info.Width != 40 || info.Height != 20 {
		t.Errorf("dims = %dx%d, want 40x20", info.Width, info.Height)
	}
	if !info.HasAlpha() {
		t.Errorf("PNG with alpha should report HasAlpha, color=%q", info.Color)
	}
	if info.Bytes != int64(len(data)) {
		t.Errorf("bytes = %d, want %d", info.Bytes, len(data))
	}
}

func TestInspectJPEG(t *testing.T) {
	data := jpegBytes(t, 64, 32)
	info, err := Inspect("t.jpg", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "jpeg" {
		t.Errorf("format = %q, want jpeg", info.Format)
	}
	if info.Width != 64 || info.Height != 32 {
		t.Errorf("dims = %dx%d, want 64x32", info.Width, info.Height)
	}
	// JPEG 是 YCbCr，无 alpha
	if info.HasAlpha() {
		t.Errorf("JPEG should not have alpha, color=%q", info.Color)
	}
}

func TestInspectGIF(t *testing.T) {
	data := gifBytes(t, 16, 8)
	info, err := Inspect("t.gif", bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "gif" {
		t.Errorf("format = %q, want gif", info.Format)
	}
	if info.Width != 16 || info.Height != 8 {
		t.Errorf("dims = %dx%d, want 16x8", info.Width, info.Height)
	}
	if info.Color != "Paletted" {
		t.Errorf("GIF color = %q, want Paletted", info.Color)
	}
}

func TestInspectNotAnImage(t *testing.T) {
	data := []byte("this is not an image")
	if _, err := Inspect("x.txt", bytes.NewReader(data), int64(len(data))); err == nil {
		t.Error("non-image data should error")
	}
}

func TestInspectTruncated(t *testing.T) {
	data := pngBytes(t, 10, 10)
	truncated := data[:8] // 只剩 PNG 魔数
	if _, err := Inspect("t.png", bytes.NewReader(truncated), int64(len(truncated))); err == nil {
		t.Error("truncated PNG should error")
	}
}

func TestColorModelName(t *testing.T) {
	cases := []struct {
		m    color.Model
		want string
	}{
		{color.RGBAModel, "RGBA"},
		{color.NRGBAModel, "NRGBA"},
		{color.GrayModel, "Gray"},
		{color.Gray16Model, "Gray16"},
		{color.CMYKModel, "CMYK"},
		{color.YCbCrModel, "YCbCr"},
		{color.Palette{color.Black}, "Paletted"},
	}
	for _, c := range cases {
		if got := ColorModelName(c.m); got != c.want {
			t.Errorf("ColorModelName = %q, want %q", got, c.want)
		}
	}
}

func TestHasAlpha(t *testing.T) {
	if !(Info{Color: "NRGBA"}).HasAlpha() {
		t.Error("NRGBA should have alpha")
	}
	if !(Info{Color: "RGBA"}).HasAlpha() {
		t.Error("RGBA should have alpha")
	}
	if (Info{Color: "YCbCr"}).HasAlpha() {
		t.Error("YCbCr should not have alpha")
	}
	if (Info{Color: "Gray"}).HasAlpha() {
		t.Error("Gray should not have alpha")
	}
}

func TestHumanizeBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := HumanizeBytes(c.n); got != c.want {
			t.Errorf("HumanizeBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
