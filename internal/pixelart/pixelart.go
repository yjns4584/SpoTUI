// Package pixelart renders images as colored Unicode block characters for terminal display.
// Uses the ▀ (upper half block) technique: each terminal cell shows two pixel rows,
// foreground color = top pixel, background color = bottom pixel.
package pixelart

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

// Render converts raw image bytes to a terminal-renderable string.
// width is the desired character width; height is derived (2:1 ratio per cell).
func Render(imgBytes []byte, width, height int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return "", err
	}
	return renderImage(img, width, height), nil
}

// RenderPixelated applies a pixelation effect before rendering —
// downscales to a coarse grid first for a stronger pixel-art look.
func RenderPixelated(imgBytes []byte, width, height, pixelSize int) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return "", err
	}
	img = pixelate(img, pixelSize)
	return renderImage(img, width, height), nil
}

func renderImage(img image.Image, charW, charH int) string {
	// Each terminal char = 2 vertical pixels (▀ trick)
	pixW := charW
	pixH := charH * 2

	scaled := resize(img, pixW, pixH)
	var sb strings.Builder

	for row := 0; row < pixH-1; row += 2 {
		for col := 0; col < pixW; col++ {
			top := scaled.At(col, row)
			bot := scaled.At(col, row+1)
			sb.WriteString(colorCell(top, bot))
		}
		sb.WriteString(resetANSI + "\n")
	}

	return sb.String()
}

const resetANSI = "\x1b[0m"

func colorCell(fg, bg color.Color) string {
	fr, fg2, fb, _ := fg.RGBA()
	br, bg2, bb, _ := bg.RGBA()
	return fmt.Sprintf("\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀",
		fr>>8, fg2>>8, fb>>8,
		br>>8, bg2>>8, bb>>8,
	)
}

// ── Image processing ─────────────────────────────────────────────────────────

type rgbaImage struct {
	pix    []color.RGBA
	w, h   int
}

func (i *rgbaImage) ColorModel() color.Model { return color.RGBAModel }
func (i *rgbaImage) Bounds() image.Rectangle { return image.Rect(0, 0, i.w, i.h) }
func (i *rgbaImage) At(x, y int) color.Color {
	if x < 0 || y < 0 || x >= i.w || y >= i.h {
		return color.RGBA{}
	}
	return i.pix[y*i.w+x]
}

func resize(img image.Image, w, h int) *rgbaImage {
	src := img.Bounds()
	srcW := src.Max.X - src.Min.X
	srcH := src.Max.Y - src.Min.Y

	out := &rgbaImage{pix: make([]color.RGBA, w*h), w: w, h: h}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// bilinear sample
			sx := float64(x) * float64(srcW) / float64(w)
			sy := float64(y) * float64(srcH) / float64(h)
			out.pix[y*w+x] = sampleBilinear(img, src.Min.X, src.Min.Y, sx, sy)
		}
	}
	return out
}

func sampleBilinear(img image.Image, ox, oy int, x, y float64) color.RGBA {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := x0+1, y0+1
	xf, yf := x-float64(x0), y-float64(y0)

	c00 := toRGBA(img.At(ox+x0, oy+y0))
	c10 := toRGBA(img.At(ox+x1, oy+y0))
	c01 := toRGBA(img.At(ox+x0, oy+y1))
	c11 := toRGBA(img.At(ox+x1, oy+y1))

	lerp := func(a, b uint8, t float64) uint8 {
		return uint8(float64(a)*(1-t) + float64(b)*t)
	}
	blend := func(c0, c1, c2, c3 color.RGBA) color.RGBA {
		top := color.RGBA{
			R: lerp(c0.R, c1.R, xf), G: lerp(c0.G, c1.G, xf),
			B: lerp(c0.B, c1.B, xf), A: lerp(c0.A, c1.A, xf),
		}
		bot := color.RGBA{
			R: lerp(c2.R, c3.R, xf), G: lerp(c2.G, c3.G, xf),
			B: lerp(c2.B, c3.B, xf), A: lerp(c2.A, c3.A, xf),
		}
		return color.RGBA{
			R: lerp(top.R, bot.R, yf), G: lerp(top.G, bot.G, yf),
			B: lerp(top.B, bot.B, yf), A: lerp(top.A, bot.A, yf),
		}
	}
	return blend(c00, c10, c01, c11)
}

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// pixelate downscales to (w/pixelSize × h/pixelSize) then upscales back,
// creating visible pixel blocks.
func pixelate(img image.Image, pixelSize int) image.Image {
	b := img.Bounds()
	w, h := b.Max.X-b.Min.X, b.Max.Y-b.Min.Y
	smallW := max(1, w/pixelSize)
	smallH := max(1, h/pixelSize)

	small := resize(img, smallW, smallH)
	// upscale back — nearest neighbor
	out := &rgbaImage{pix: make([]color.RGBA, w*h), w: w, h: h}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := x * smallW / w
			sy := y * smallH / h
			out.pix[y*w+x] = small.pix[sy*smallW+sx]
		}
	}
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
