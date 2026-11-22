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
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

// gradientID is fixed so the SVG is deterministic; the editor shows SVGs through <img>, where
// ids cannot collide.
const gradientID = "qr-fill"

// paint is a solid #rrggbb color or the shared gradient.
type paint struct {
	Color    string
	Gradient bool
}

func (p paint) svg() string {
	if p.Gradient {
		return "url(#" + gradientID + ")"
	}
	return p.Color
}

type layer struct {
	Contours []contour
	Fill     paint
}

type box struct{ X, Y, W float64 }

// design is the format-independent drawing: everything in module units, origin at the
// top-left of the quiet zone.
type design struct {
	N, Q, Total int
	Background  string // empty when transparent
	Style       Style
	Layers      []layer
	Tile        *box // opaque backing behind a clean logo
	Logo        *box // logo placement
}

// Matrix encodes the payload exactly as Render does: M correction, or H when a logo is used.
// Rows are strings of '0'/'1' so the browser engine can draw the same modules.
func Matrix(payload string, withLogo bool) ([]string, error) {
	m, err := encode(payload, withLogo)
	if err != nil {
		return nil, err
	}
	rows := make([]string, len(m))
	for y, row := range m {
		b := make([]byte, len(row))
		for x, on := range row {
			b[x] = '0'
			if on {
				b[x] = '1'
			}
		}
		rows[y] = string(b)
	}
	return rows, nil
}

// ParseMatrix is the inverse of Matrix.
func ParseMatrix(rows []string) ([][]bool, error) {
	m := make([][]bool, len(rows))
	for y, row := range rows {
		if len(row) != len(rows) {
			return nil, ErrInvalid
		}
		m[y] = make([]bool, len(row))
		for x := range row {
			switch row[x] {
			case '1':
				m[y][x] = true
			case '0':
			default:
				return nil, ErrInvalid
			}
		}
	}
	if len(m) < 21 {
		return nil, ErrInvalid
	}
	return m, nil
}

func encode(payload string, withLogo bool) ([][]bool, error) {
	level := qrcode.Medium
	if withLogo {
		level = qrcode.High
	}
	code, err := qrcode.New(payload, level)
	if err != nil {
		return nil, fmt.Errorf("encode QR: %w", err)
	}
	code.DisableBorder = true
	return code.Bitmap(), nil
}

func buildDesign(m [][]bool, s Style) design {
	n, q := len(m), s.QuietZone
	d := design{N: n, Q: q, Total: n + 2*q, Style: s}
	if !s.TransparentBackground {
		d.Background = s.Background
	}
	hasLogo := s.LogoProfileUUID != nil
	cleared := func(int, int) bool { return false }
	if hasLogo {
		w := float64(n*s.LogoSize) / 100
		d.Logo = &box{X: (float64(n)-w)/2 + float64(q), Y: (float64(n)-w)/2 + float64(q), W: w}
		if s.LogoMode == LogoClean {
			tw := w + 2
			t0 := (float64(n) - tw) / 2
			t1 := t0 + tw
			if s.TransparentBackground {
				// Nothing can cover modules on a transparent background: drop the ones whose
				// center lies under the tile (the same area an opaque tile covers).
				cleared = func(x, y int) bool {
					cx, cy := float64(x)+.5, float64(y)+.5
					return cx > t0 && cx < t1 && cy > t0 && cy < t1
				}
			} else {
				// Partly covered modules stay and are clipped by the opaque tile, as in v1.
				cleared = func(x, y int) bool {
					fx, fy := float64(x), float64(y)
					return fx >= t0 && fx+1 <= t1 && fy >= t0 && fy+1 <= t1
				}
				d.Tile = &box{X: t0 + float64(q), Y: t0 + float64(q), W: tw}
			}
		}
	}
	g := grid{n: n}
	g.off = func(x, y int) bool { return inFinder(n, x, y) || cleared(x, y) }
	g.on = func(x, y int) bool {
		return x >= 0 && y >= 0 && x < n && y < n && m[y][x] && !g.off(x, y)
	}
	gradient := s.FillMode != FillSolid
	d.Layers = append(d.Layers, layer{bodyGeometry(g, s.BodyShape, float64(q)), paint{Color: s.Foreground, Gradient: gradient}})
	eyePaint := func(c string) paint {
		if c != "" {
			return paint{Color: c}
		}
		return paint{Color: s.Foreground, Gradient: gradient && s.GradientOnEyes}
	}
	frame, ball := eyeFrame(s.EyeFrameShape), eyeBall(s.EyeBallShape)
	for i, pos := range [3][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		e := s.Eyes[i]
		ox, oy := float64(pos[0]+q), float64(pos[1]+q)
		d.Layers = append(d.Layers,
			layer{placeEye(frame, ox, oy, e), eyePaint(e.FrameColor)},
			layer{placeEye(ball, ox, oy, e), eyePaint(e.BallColor)},
		)
	}
	return d
}

// center and radialRadius place the gradient over the symbol (quiet zone excluded).
func (d design) center() float64       { return float64(d.Q) + float64(d.N)/2 }
func (d design) radialRadius() float64 { return mul(float64(d.N), 0.7071067811865476) }

// RenderSVG draws a matrix with the shared geometry. It is the function the golden fixtures
// pin; logoHref is the image reference used when the style has a logo (a data: URL).
func RenderSVG(m [][]bool, s Style, size int, logoHref string) string {
	return svgDocument(buildDesign(m, s.Normalize()), size, logoHref)
}

func escapeAttr(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(v)
}

func svgDocument(d design, size int, logoHref string) string {
	s := d.Style
	t := fmtNum(float64(d.Total))
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %s %s">`, size, size, t, t)
	if s.FillMode != FillSolid {
		stops := `<stop offset="0" stop-color="` + s.GradientStart + `"/><stop offset="1" stop-color="` + s.GradientEnd + `"/>`
		c := fmtNum(d.center())
		if s.FillMode == FillLinear {
			sb.WriteString(`<defs><linearGradient id="` + gradientID + `" gradientUnits="userSpaceOnUse" x1="` + fmtNum(float64(d.Q)) +
				`" y1="` + c + `" x2="` + fmtNum(float64(d.Q+d.N)) + `" y2="` + c + `" gradientTransform="rotate(` +
				fmtNum(float64(s.GradientAngle)) + ` ` + c + ` ` + c + `)">` + stops + `</linearGradient></defs>`)
		} else {
			sb.WriteString(`<defs><radialGradient id="` + gradientID + `" gradientUnits="userSpaceOnUse" cx="` + c + `" cy="` + c +
				`" r="` + fmtNum(d.radialRadius()) + `">` + stops + `</radialGradient></defs>`)
		}
	}
	if d.Background != "" {
		sb.WriteString(`<rect width="` + t + `" height="` + t + `" fill="` + d.Background + `"/>`)
	}
	for _, l := range d.Layers {
		if len(l.Contours) == 0 {
			continue
		}
		sb.WriteString(`<path fill="` + l.Fill.svg() + `" d="` + pathData(l.Contours) + `"/>`)
	}
	if d.Tile != nil {
		sb.WriteString(`<rect x="` + fmtNum(d.Tile.X) + `" y="` + fmtNum(d.Tile.Y) + `" width="` + fmtNum(d.Tile.W) +
			`" height="` + fmtNum(d.Tile.W) + `" fill="` + s.Background + `"/>`)
	}
	if d.Logo != nil && logoHref != "" {
		sb.WriteString(`<image x="` + fmtNum(d.Logo.X) + `" y="` + fmtNum(d.Logo.Y) + `" width="` + fmtNum(d.Logo.W) +
			`" height="` + fmtNum(d.Logo.W) + `" preserveAspectRatio="xMidYMid meet" href="` + escapeAttr(logoHref) + `"/>`)
	}
	sb.WriteString("</svg>")
	return sb.String()
}

// Render draws the payload in svg, png or pdf. Finder patterns keep their 7/5/3 proportions,
// a logo selects H correction, and every format shares one geometry.
func Render(payload, format string, size int, style Style, logo image.Image) ([]byte, string, error) {
	style = style.Normalize()
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
	hasLogo := style.LogoProfileUUID != nil
	if hasLogo && logo == nil {
		return nil, "", ErrInvalid
	}
	m, err := encode(payload, hasLogo)
	if err != nil {
		return nil, "", err
	}
	if size < (len(m)+2*style.QuietZone)*4 {
		return nil, "", ErrInvalid
	}
	d := buildDesign(m, style)
	switch format {
	case "png":
		var out bytes.Buffer
		if err := png.Encode(&out, rasterize(d, size, logo)); err != nil {
			return nil, "", err
		}
		return out.Bytes(), "image/png", nil
	case "pdf":
		data, err := renderPDF(d, size, logo)
		return data, "application/pdf", err
	}
	href := ""
	if hasLogo {
		var b bytes.Buffer
		if err := png.Encode(&b, logoTile(d, logo, int(math.Max(1, math.Round(d.Logo.W*float64(size)/float64(d.Total)))))); err != nil {
			return nil, "", err
		}
		href = "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
	}
	return []byte(svgDocument(d, size, href)), "image/svg+xml", nil
}

// logoTile fits the logo (aspect kept, centered) on a square of the given pixel width. The
// square is the background color for a clean logo on an opaque background, so transparent
// logo pixels never reveal modules; otherwise it is transparent.
func logoTile(d design, src image.Image, width int) *image.RGBA {
	tile := image.NewRGBA(image.Rect(0, 0, width, width))
	if d.Tile != nil {
		draw.Draw(tile, tile.Bounds(), image.NewUniform(parseColor(d.Background)), image.Point{}, draw.Src)
	}
	b := src.Bounds()
	ratio := math.Min(float64(width)/float64(b.Dx()), float64(width)/float64(b.Dy()))
	w, h := int(math.Round(float64(b.Dx())*ratio)), int(math.Round(float64(b.Dy())*ratio))
	w, h = max(w, 1), max(h, 1)
	x, y := (width-w)/2, (width-h)/2
	xdraw.CatmullRom.Scale(tile, image.Rect(x, y, x+w, y+h), src, b, draw.Over, nil)
	return tile
}

// gradientAt returns the fill color at a point in module units, matching the SVG gradient:
// linear runs across the symbol width rotated by the angle (clockwise, 0 = left to right),
// radial runs from the center to the symbol corners. Both pad outside 0..1.
func gradientAt(d design, x, y float64) color.RGBA {
	s := d.Style
	c := d.center()
	var t float64
	if s.FillMode == FillLinear {
		a := float64(s.GradientAngle) * math.Pi / 180
		t = ((x-c)*math.Cos(a)+(y-c)*math.Sin(a))/float64(d.N) + .5
	} else {
		t = math.Hypot(x-c, y-c) / d.radialRadius()
	}
	t = math.Max(0, math.Min(1, t))
	a, b := parseColor(s.GradientStart), parseColor(s.GradientEnd)
	lerp := func(u, v uint8) uint8 { return uint8(math.Round(float64(u) + (float64(v)-float64(u))*t)) }
	return color.RGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: 255}
}

// rasterize draws an anti-aliased PNG with golang.org/x/image/vector (non-zero winding, so
// overlapping pieces merge and counter-clockwise holes cut out, as in the SVG).
func rasterize(d design, size int, logo image.Image) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	if d.Background != "" {
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(parseColor(d.Background)), image.Point{}, draw.Src)
	}
	k := float64(size) / float64(d.Total)
	var grad *image.RGBA
	z := vector.NewRasterizer(size, size)
	for _, l := range d.Layers {
		if len(l.Contours) == 0 {
			continue
		}
		z.Reset(size, size)
		for _, c := range l.Contours {
			z.MoveTo(float32(c.Start.X*k), float32(c.Start.Y*k))
			for _, s := range c.Segs {
				if s.Cubic {
					z.CubeTo(float32(s.C1.X*k), float32(s.C1.Y*k), float32(s.C2.X*k), float32(s.C2.Y*k), float32(s.P.X*k), float32(s.P.Y*k))
				} else {
					z.LineTo(float32(s.P.X*k), float32(s.P.Y*k))
				}
			}
			z.ClosePath()
		}
		var src image.Image = image.NewUniform(parseColor(l.Fill.Color))
		if l.Fill.Gradient {
			if grad == nil {
				grad = image.NewRGBA(canvas.Bounds())
				for py := 0; py < size; py++ {
					for px := 0; px < size; px++ {
						grad.SetRGBA(px, py, gradientAt(d, (float64(px)+.5)/k, (float64(py)+.5)/k))
					}
				}
			}
			src = grad
		}
		z.Draw(canvas, canvas.Bounds(), src, image.Point{})
	}
	if d.Tile != nil {
		x0, y0, x1 := int(math.Round(d.Tile.X*k)), int(math.Round(d.Tile.Y*k)), int(math.Round((d.Tile.X+d.Tile.W)*k))
		draw.Draw(canvas, image.Rect(x0, y0, x1, y0+(x1-x0)), image.NewUniform(parseColor(d.Background)), image.Point{}, draw.Src)
	}
	if d.Logo != nil && logo != nil {
		x0, y0 := int(math.Round(d.Logo.X*k)), int(math.Round(d.Logo.Y*k))
		w := max(int(math.Round(d.Logo.W*k)), 1)
		tile := logoTile(d, logo, w)
		draw.Draw(canvas, image.Rect(x0, y0, x0+w, y0+w), tile, image.Point{}, draw.Over)
	}
	return canvas
}

// renderPDF places the vector design on A4 (160 mm square at 25 mm). Solid layers are filled
// paths; gradient layers clip a PDF shading to the same path. The logo is embedded as PNG at
// the requested raster resolution.
func renderPDF(d design, size int, logo image.Image) ([]byte, error) {
	const origin, edge = 25.0, 160.0
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(true)
	pdf.AddPage()
	u := edge / float64(d.Total)
	mm := func(p pt) (float64, float64) { return origin + p.X*u, origin + p.Y*u }
	fill := func(hex string) {
		c := parseColor(hex)
		pdf.SetFillColor(int(c.R), int(c.G), int(c.B))
	}
	tracePath := func(cs []contour) {
		for _, c := range cs {
			pdf.MoveTo(mm(c.Start))
			for _, s := range c.Segs {
				if s.Cubic {
					x1, y1 := mm(s.C1)
					x2, y2 := mm(s.C2)
					x3, y3 := mm(s.P)
					pdf.CurveBezierCubicTo(x1, y1, x2, y2, x3, y3)
				} else {
					pdf.LineTo(mm(s.P))
				}
			}
			pdf.ClosePath()
		}
	}
	if d.Background != "" {
		fill(d.Background)
		pdf.Rect(origin, origin, edge, edge, "F")
	}
	s := d.Style
	for _, l := range d.Layers {
		if len(l.Contours) == 0 {
			continue
		}
		if !l.Fill.Gradient {
			fill(l.Fill.Color)
			tracePath(l.Contours)
			pdf.DrawPath("f")
			continue
		}
		a, b := parseColor(s.GradientStart), parseColor(s.GradientEnd)
		x, y := mm(pt{float64(d.Q), float64(d.Q)})
		w := float64(d.N) * u
		pdf.RawWriteStr("q")
		tracePath(l.Contours)
		pdf.RawWriteStr("W n")
		if s.FillMode == FillLinear {
			// Normalized shading coordinates have y up; the style angle is clockwise on screen.
			rad := float64(s.GradientAngle) * math.Pi / 180
			cx, cy := math.Cos(rad)/2, math.Sin(rad)/2
			pdf.LinearGradient(x, y, w, w, int(a.R), int(a.G), int(a.B), int(b.R), int(b.G), int(b.B), .5-cx, .5+cy, .5+cx, .5-cy)
		} else {
			pdf.RadialGradient(x, y, w, w, int(a.R), int(a.G), int(a.B), int(b.R), int(b.G), int(b.B), .5, .5, .5, .5, d.radialRadius()/float64(d.N))
		}
		pdf.RawWriteStr("Q")
	}
	if d.Tile != nil {
		fill(d.Background)
		x, y := mm(pt{d.Tile.X, d.Tile.Y})
		pdf.Rect(x, y, d.Tile.W*u, d.Tile.W*u, "F")
	}
	if d.Logo != nil && logo != nil {
		px := max(int(math.Round(d.Logo.W*float64(size)/float64(d.Total))), 1)
		var b bytes.Buffer
		if err := png.Encode(&b, logoTile(d, logo, px)); err != nil {
			return nil, err
		}
		opts := gofpdf.ImageOptions{ImageType: "PNG"}
		pdf.RegisterImageOptionsReader("logo", opts, &b)
		x, y := mm(pt{d.Logo.X, d.Logo.Y})
		pdf.ImageOptions("logo", x, y, d.Logo.W*u, d.Logo.W*u, false, opts, 0, "")
	}
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
