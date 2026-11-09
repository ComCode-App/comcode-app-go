package qr

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"

	"github.com/jung-kurt/gofpdf/v2"
	qrcode "github.com/skip2/go-qrcode"
)

type shape struct {
	x, y, w, r float64
	fill       string
}

// Render uses one geometry for all formats. Finder patterns stay intact; only the
// center 15% is covered by a logo, with H correction and an opaque backing.
func Render(payload, format string, size int, style Style, logo image.Image) ([]byte, string, error) {
	if err := style.Validate(); err != nil {
		return nil, "", err
	}
	if size < 256 || size > 2048 {
		return nil, "", ErrInvalid
	}
	if format != "svg" && format != "png" && format != "pdf" {
		return nil, "", ErrInvalid
	}
	if logo != nil && (logo.Bounds().Dx() <= 0 || logo.Bounds().Dy() <= 0) {
		return nil, "", ErrInvalid
	}
	level := qrcode.Medium
	if style.LogoProfileUUID != nil {
		level = qrcode.High
		if logo == nil {
			return nil, "", ErrInvalid
		}
	}
	code, err := qrcode.New(payload, level)
	if err != nil {
		return nil, "", fmt.Errorf("encode QR: %w", err)
	}
	code.DisableBorder = true
	matrix := code.Bitmap()
	n := len(matrix)
	total := n + 2*style.QuietZone
	if size < total*4 {
		return nil, "", ErrInvalid
	}
	shapes := geometry(matrix, style)
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(parseColor(style.Background)), image.Point{}, draw.Src)
	scale := float64(size) / float64(total)
	for _, sh := range shapes {
		paint(canvas, sh, scale)
	}
	var logoPNG []byte
	if logo != nil && style.LogoProfileUUID != nil {
		width := int(float64(n) * .15 * scale)
		logoPNG = placeLogo(canvas, logo, width, parseColor(style.Background))
	}
	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, "", err
	}
	switch format {
	case "png":
		return out.Bytes(), "image/png", nil
	case "pdf":
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.RegisterImageOptionsReader("qr", gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(out.Bytes()))
		pdf.ImageOptions("qr", 25, 25, 160, 160, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		out.Reset()
		if err := pdf.Output(&out); err != nil {
			return nil, "", err
		}
		return out.Bytes(), "application/pdf", nil
	}
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><rect width="100%%" height="100%%" fill="%s"/>`, size, size, total, total, style.Background)
	for _, sh := range shapes {
		fmt.Fprintf(&svg, `<rect x="%g" y="%g" width="%g" height="%g" rx="%g" fill="%s"/>`, sh.x, sh.y, sh.w, sh.w, sh.r, sh.fill)
	}
	if len(logoPNG) > 0 {
		w := float64(n) * .15
		pos := (float64(total) - w) / 2
		fmt.Fprintf(&svg, `<image x="%g" y="%g" width="%g" height="%g" href="data:image/png;base64,%s"/>`, pos, pos, w, w, base64.StdEncoding.EncodeToString(logoPNG))
	}
	svg.WriteString("</svg>")
	return []byte(svg.String()), "image/svg+xml", nil
}

func geometry(m [][]bool, s Style) []shape {
	n, q := len(m), float64(s.QuietZone)
	shapes := make([]shape, 0, n*n/2)
	radius := 0.0
	switch s.Module {
	case "rounded":
		radius = .2
	case "dots":
		radius = .5
	}
	for y, row := range m {
		for x, on := range row {
			finder := (x < 7 && y < 7) || (x >= n-7 && y < 7) || (x < 7 && y >= n-7)
			if on && !finder {
				shapes = append(shapes, shape{x: float64(x) + q, y: float64(y) + q, w: 1, r: radius, fill: s.Foreground})
			}
		}
	}
	for _, pos := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for i, w := range []float64{7, 5, 3} {
			r := 0.0
			switch s.Eye {
			case "rounded":
				r = math.Min(1, w/2)
			case "circle":
				r = w / 2
			}
			fill := s.Foreground
			if i == 1 {
				fill = s.Background
			}
			shapes = append(shapes, shape{x: float64(pos[0]+i) + q, y: float64(pos[1]+i) + q, w: w, r: r, fill: fill})
		}
	}
	if s.LogoProfileUUID != nil {
		w := float64(n)*.15 + 2
		shapes = append(shapes, shape{x: (float64(n) + 2*q - w) / 2, y: (float64(n) + 2*q - w) / 2, w: w, fill: s.Background})
	}
	return shapes
}

func paint(dst *image.RGBA, s shape, scale float64) {
	fill := parseColor(s.fill)
	x0, y0, w, r := s.x*scale, s.y*scale, s.w*scale, s.r*scale
	for y := int(math.Floor(y0)); y < int(math.Ceil(y0+w)); y++ {
		for x := int(math.Floor(x0)); x < int(math.Ceil(x0+w)); x++ {
			px, py := float64(x)+.5, float64(y)+.5
			if px < x0 || py < y0 || px >= x0+w || py >= y0+w {
				continue
			}
			dx := math.Max(math.Max(x0+r-px, px-(x0+w-r)), 0)
			dy := math.Max(math.Max(y0+r-py, py-(y0+w-r)), 0)
			if dx*dx+dy*dy <= r*r {
				dst.SetRGBA(x, y, fill)
			}
		}
	}
}

func placeLogo(dst *image.RGBA, src image.Image, width int, background color.RGBA) []byte {
	// Fit the original aspect ratio on an opaque square; normalize to PNG in every output.
	tile := image.NewRGBA(image.Rect(0, 0, width, width))
	draw.Draw(tile, tile.Bounds(), image.NewUniform(background), image.Point{}, draw.Src)
	overlay := image.NewRGBA(tile.Bounds())
	bounds := src.Bounds()
	ratio := math.Min(float64(width)/float64(bounds.Dx()), float64(width)/float64(bounds.Dy()))
	w, h := int(float64(bounds.Dx())*ratio), int(float64(bounds.Dy())*ratio)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			overlay.Set((width-w)/2+x, (width-h)/2+y, src.At(bounds.Min.X+int(float64(x)/ratio), bounds.Min.Y+int(float64(y)/ratio)))
		}
	}
	draw.Draw(tile, tile.Bounds(), overlay, image.Point{}, draw.Over)
	pos := (dst.Bounds().Dx() - width) / 2
	draw.Draw(dst, image.Rect(pos, pos, pos+width, pos+width), tile, image.Point{}, draw.Src)
	var b bytes.Buffer
	_ = png.Encode(&b, tile)
	return b.Bytes()
}
