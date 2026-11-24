package qr

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/google/uuid"
	qrcode "github.com/skip2/go-qrcode"
)

func TestStyleValidation(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Style)
	}{
		{"color injection", func(s *Style) { s.Foreground = `#000000\"/>` }},
		{"low contrast", func(s *Style) { s.Foreground = "#aaaaaa" }},
		{"inverted", func(s *Style) { s.Foreground = "#ffffff"; s.Background = "#000000" }},
		{"missing quiet zone", func(s *Style) { s.QuietZone = 3 }},
		{"large quiet zone", func(s *Style) { s.QuietZone = 17 }},
		{"unknown module", func(s *Style) { s.Module = "triangle" }},
		{"unknown eye", func(s *Style) { s.Eye = "triangle" }},
		{"logo field without logo", func(s *Style) { s.LogoField = "avatar" }},
		{"zero logo UUID", func(s *Style) { id := uuid.Nil; s.LogoProfileUUID = &id; s.LogoField = "avatar" }},
		{"unknown logo field", func(s *Style) { id := uuid.New(); s.LogoProfileUUID = &id; s.LogoField = "url" }},
	}
	if err := DefaultStyle().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := DefaultStyle()
			tt.change(&s)
			if s.Validate() == nil {
				t.Fatal("accepted invalid style")
			}
		})
	}
}

// go-qrcode is an encoder only. Verify every default PNG module against the
// library's matrix for the exact identity payload, including its quiet zone.
func TestPNGEncodesIdentityPayload(t *testing.T) {
	payload := PayloadURL("https://comcode.test/", "c/abc123")
	if payload != "https://comcode.test/c/abc123?src=qr" {
		t.Fatal(payload)
	}
	code, err := qrcode.New(payload, qrcode.Medium)
	if err != nil {
		t.Fatal(err)
	}
	matrix := code.Bitmap()
	size := len(matrix) * 8
	// Square shapes fill every module center, so each sampled pixel is exactly one bit.
	data, ct, err := Render(payload, "png", size, legacyStyle().Normalize(), nil)
	if err != nil || ct != "image/png" {
		t.Fatalf("render: %s %v", ct, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for y, row := range matrix {
		for x, on := range row {
			r, _, _, _ := img.At(x*8+4, y*8+4).RGBA()
			if (r == 0) != on {
				t.Fatalf("wrong payload module %d,%d", x, y)
			}
		}
	}
}

func TestRenderingVariantsAndFormats(t *testing.T) {
	for _, module := range []string{"square", "rounded", "dots"} {
		for _, eye := range []string{"square", "rounded", "circle"} {
			t.Run(module+"/"+eye, func(t *testing.T) {
				s := DefaultStyle()
				s.Module = module
				s.Eye = eye
				for _, format := range []string{"png", "svg", "pdf"} {
					data, _, err := Render("https://comcode.test/s/example?src=qr", format, 512, s, nil)
					if err != nil {
						t.Fatal(err)
					}
					switch format {
					case "png":
						img, err := png.Decode(bytes.NewReader(data))
						if err != nil || img.Bounds().Dx() != 512 {
							t.Fatalf("bad PNG: %v", err)
						}
					case "svg":
						var node struct{ XMLName xml.Name }
						if err := xml.Unmarshal(data, &node); err != nil || node.XMLName.Local != "svg" {
							t.Fatalf("bad SVG: %v", err)
						}
					case "pdf":
						if !bytes.HasPrefix(data, []byte("%PDF-")) {
							t.Fatal("bad PDF")
						}
					}
				}
			})
		}
	}
}

func TestLogoAndRenderLimits(t *testing.T) {
	s := DefaultStyle()
	id := uuid.New()
	s.LogoProfileUUID = &id
	s.LogoField = "avatar"
	if _, _, err := Render("test", "png", 512, s, nil); err == nil {
		t.Fatal("missing logo accepted")
	}
	logo := image.NewRGBA(image.Rect(0, 0, 40, 20))
	logo.Set(20, 10, color.Black)
	data, _, err := Render("https://comcode.test/c/test", "svg", 512, s, logo)
	if err != nil || !strings.Contains(string(data), "data:image/png;base64,") {
		t.Fatalf("logo: %v", err)
	}
	for _, size := range []int{0, 255, 2049} {
		if _, _, err := Render("test", "png", size, DefaultStyle(), nil); err == nil {
			t.Fatalf("accepted size %d", size)
		}
	}
	if _, _, err := Render("test", "gif", 512, DefaultStyle(), nil); err == nil {
		t.Fatal("accepted GIF")
	}
}

func TestURLBuildersShareRouteIdentity(t *testing.T) {
	base, code := "https://comcode.test/", "cc-abc123"
	if CardIdentityURL(base, code) != "https://comcode.test/c/cc-abc123" {
		t.Fatal("NFC identity changed")
	}
	if CardPayloadURL(base, code) != "https://comcode.test/c/cc-abc123?src=qr" {
		t.Fatal("manifest QR must encode marked public identity")
	}
	for _, path := range []string{"s/example", "username", "a8f9c2d1"} {
		if PayloadURL(base, path) != "https://comcode.test/"+path+"?src=qr" {
			t.Fatal("route QR must encode marked public identity")
		}
	}
	if got, want := CardPayloadURL(base, code), PayloadURL(base, "c/"+code); got != want {
		t.Fatalf("manifest QR %s != route QR %s", got, want)
	}
	if CardIdentityURL(base, "x/y") != "https://comcode.test/c/x%2Fy" {
		t.Fatal("NFC escaping changed")
	}
}

func TestTransparentLogoKeepsOpaqueBacking(t *testing.T) {
	s := DefaultStyle()
	s.Background = "#ffeecc"
	id := uuid.New()
	s.LogoProfileUUID = &id
	s.LogoField = "avatar"
	data, _, err := Render("https://comcode.test/c/test?src=qr", "png", 512, s, image.NewRGBA(image.Rect(0, 0, 32, 32)))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.RGBAModel.Convert(img.At(256, 256)); got != parseColor(s.Background) {
		t.Fatalf("transparent logo removed opaque background: %v", got)
	}
}

func TestStyleV2Validation(t *testing.T) {
	logoID := uuid.New()
	tests := []struct {
		name   string
		change func(*Style)
		ok     bool
	}{
		{"v2 shapes", func(s *Style) { s.BodyShape, s.EyeFrameShape, s.EyeBallShape = "leaf", "dotted", "clover" }, true},
		{"unknown body", func(s *Style) { s.BodyShape = "frame14" }, false},
		{"unknown frame", func(s *Style) { s.EyeFrameShape = "ball16" }, false},
		{"unknown ball", func(s *Style) { s.EyeBallShape = "triangle" }, false},
		{"linear gradient", func(s *Style) {
			s.FillMode, s.GradientStart, s.GradientEnd, s.GradientAngle = FillLinear, "#000000", "#1e3a8a", 90
		}, true},
		{"gradient end too light", func(s *Style) { s.FillMode, s.GradientStart, s.GradientEnd = FillRadial, "#000000", "#cccccc" }, false},
		{"gradient start too light", func(s *Style) { s.FillMode, s.GradientStart, s.GradientEnd = FillLinear, "#dddddd", "#000000" }, false},
		{"light gradient color unused when solid", func(s *Style) { s.GradientEnd = "#cccccc" }, true},
		{"gradient color injection", func(s *Style) { s.GradientEnd = `#000"/>` }, false},
		{"unknown fill", func(s *Style) { s.FillMode = "conic" }, false},
		{"angle range", func(s *Style) { s.FillMode, s.GradientAngle = FillLinear, 360 }, false},
		{"eye colors", func(s *Style) { s.Eyes[0].FrameColor, s.Eyes[2].BallColor = "#7c2d12", "#1e3a8a" }, true},
		{"light eye frame", func(s *Style) { s.Eyes[1].FrameColor = "#eeeeee" }, false},
		{"inverted eye ball", func(s *Style) { s.Background, s.Foreground, s.Eyes[2].BallColor = "#fffbeb", "#000000", "#ffffff" }, false},
		{"eye color injection", func(s *Style) { s.Eyes[0].BallColor = "red" }, false},
		{"transparent judged on white", func(s *Style) { s.TransparentBackground, s.Background = true, "#000000" }, true},
		{"transparent still needs dark modules", func(s *Style) { s.TransparentBackground, s.Foreground = true, "#bbbbbb" }, false},
		{"logo size min", func(s *Style) { s.LogoSize = 9 }, false},
		{"logo size max", func(s *Style) { s.LogoProfileUUID, s.LogoField, s.LogoSize = &logoID, "cover", 31 }, false},
		{"logo max size and overlay", func(s *Style) {
			s.LogoProfileUUID, s.LogoField, s.LogoSize, s.LogoMode = &logoID, "cover", 30, LogoOverlay
		}, true},
		{"unknown logo mode", func(s *Style) { s.LogoMode = "default" }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := DefaultStyle()
			tt.change(&s)
			if err := s.Validate(); (err == nil) != tt.ok {
				t.Fatalf("Validate() = %v, want ok=%v", err, tt.ok)
			}
		})
	}
}

func TestNormalizeMapsV1Fields(t *testing.T) {
	s, err := ParseStored([]byte(`{"foreground":"#1F2937","background":"#ffffff","module":"dots","eye":"rounded","quiet_zone":6}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.BodyShape != "circle" || s.EyeFrameShape != "rounded" || s.EyeBallShape != "rounded" || s.Foreground != "#1f2937" ||
		s.FillMode != FillSolid || s.LogoMode != LogoClean || s.LogoSize != LogoSizeDefault || s.QuietZone != 6 {
		t.Fatalf("v1 mapping: %+v", s)
	}
	for _, raw := range []string{`{}`, ``, ` { } `} {
		empty, err := ParseStored([]byte(raw))
		if err != nil || empty != DefaultStyle() {
			t.Fatalf("column default %q must be the default style: %+v %v", raw, empty, err)
		}
	}
	if d := DefaultStyle(); d.BodyShape != "smooth" || d.EyeFrameShape != "rounded-large" || d.EyeBallShape != "rounded" ||
		d.Foreground != "#000000" || d.Background != "#ffffff" || d.QuietZone != 4 || d.LogoProfileUUID != nil || d.Validate() != nil {
		t.Fatalf("default design (TEC-1273): %+v", d)
	}
	// A style saved before the default changed keeps its look, even when it only names the old defaults.
	saved, err := ParseStored([]byte(`{"foreground":"#000000","background":"#ffffff","module":"square","eye":"square","quiet_zone":4}`))
	if err != nil || saved.BodyShape != "square" || saved.EyeFrameShape != "square" || saved.EyeBallShape != "square" {
		t.Fatalf("saved v1 square style: %+v %v", saved, err)
	}
	if partial, err := ParseStored([]byte(`{"foreground":"#1f2937"}`)); err != nil || partial.BodyShape != "square" {
		t.Fatalf("a saved partial style decodes over v1 defaults: %+v %v", partial, err)
	}
	// v2-only clients get the v1 fields derived, so older readers keep working.
	v2 := Style{Foreground: "#000000", Background: "#ffffff", BodyShape: "rounded-square", EyeFrameShape: "circle", EyeBallShape: "circle", QuietZone: 4}.Normalize()
	if v2.Module != "rounded" || v2.Eye != "circle" {
		t.Fatalf("v1 fields from v2: %+v", v2)
	}
	if other := (Style{Foreground: "#000000", Background: "#ffffff", BodyShape: "leaf", QuietZone: 4}).Normalize(); other.Module != "square" || other.EyeFrameShape != "square" {
		t.Fatalf("v2 defaults: %+v", other)
	}
}

// The browser engine draws GET /qr's matrices; for the same style its SVG must equal /qr/render.
func TestRenderSVGMatchesServerMatrix(t *testing.T) {
	payload := PayloadURL("https://comcode.test", "c/cc-89a12b")
	s := DefaultStyle()
	s.BodyShape, s.FillMode, s.GradientStart, s.GradientEnd, s.GradientAngle = "rounded-in-soft", FillLinear, "#000000", "#1e3a8a", 30
	rows, err := Matrix(payload, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMatrix(rows)
	if err != nil {
		t.Fatal(err)
	}
	server, _, err := Render(payload, "svg", 1024, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if RenderSVG(m, s, 1024, "") != string(server) {
		t.Fatal("matrix render differs from server SVG")
	}
	// With a logo only the embedded image differs: the browser embeds its own copy.
	id := uuid.New()
	s.LogoProfileUUID, s.LogoField = &id, "avatar"
	rows, _ = Matrix(payload, true)
	m, _ = ParseMatrix(rows)
	server, _, err = Render(payload, "svg", 1024, s, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(server), `href="`)
	if i < 0 || RenderSVG(m, s, 1024, "x")[:i] != string(server)[:i] {
		t.Fatal("logo matrix render differs from server SVG")
	}
	if _, err := ParseMatrix([]string{"01", "10"}); err == nil {
		t.Fatal("accepted a tiny matrix")
	}
}

func TestPDFIsVectorWithGradient(t *testing.T) {
	s := DefaultStyle()
	s.FillMode, s.GradientStart, s.GradientEnd = FillRadial, "#000000", "#1e3a8a"
	s.TransparentBackground = true
	data, ct, err := Render("https://comcode.test/c/x?src=qr", "pdf", 512, s, nil)
	if err != nil || ct != "application/pdf" || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("pdf: %v", err)
	}
	if !bytes.Contains(data, []byte("/ShadingType 3")) || bytes.Contains(data, []byte("/Subtype /Image")) {
		t.Fatal("gradient PDF must use a vector shading, not an embedded raster")
	}
}

// legacyRender is the v1 renderer (before TEC-1269), kept to prove v1 styles look the same.
func legacyRender(m [][]bool, s Style, size int) *image.RGBA {
	n, q := len(m), float64(s.QuietZone)
	type box struct {
		x, y, w, r float64
		fill       string
	}
	var shapes []box
	radius := map[string]float64{"square": 0, "rounded": .2, "dots": .5}[s.Module]
	for y, row := range m {
		for x, on := range row {
			if on && !inFinder(n, x, y) {
				shapes = append(shapes, box{float64(x) + q, float64(y) + q, 1, radius, s.Foreground})
			}
		}
	}
	for _, pos := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for i, w := range []float64{7, 5, 3} {
			r := map[string]float64{"square": 0, "rounded": math.Min(1, w/2), "circle": w / 2}[s.Eye]
			fill := s.Foreground
			if i == 1 {
				fill = s.Background
			}
			shapes = append(shapes, box{float64(pos[0]+i) + q, float64(pos[1]+i) + q, w, r, fill})
		}
	}
	if s.LogoProfileUUID != nil {
		w := float64(n)*.15 + 2
		shapes = append(shapes, box{(float64(n) + 2*q - w) / 2, (float64(n) + 2*q - w) / 2, w, 0, s.Background})
	}
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(parseColor(s.Background)), image.Point{}, draw.Src)
	scale := float64(size) / (float64(n) + 2*q)
	for _, sh := range shapes {
		x0, y0, w, r := sh.x*scale, sh.y*scale, sh.w*scale, sh.r*scale
		for y := int(y0); y < int(math.Ceil(y0+w)); y++ {
			for x := int(x0); x < int(math.Ceil(x0+w)); x++ {
				px, py := float64(x)+.5, float64(y)+.5
				if px < x0 || py < y0 || px >= x0+w || py >= y0+w {
					continue
				}
				dx := math.Max(math.Max(x0+r-px, px-(x0+w-r)), 0)
				dy := math.Max(math.Max(y0+r-py, py-(y0+w-r)), 0)
				if dx*dx+dy*dy <= r*r {
					canvas.SetRGBA(x, y, parseColor(sh.fill))
				}
			}
		}
	}
	return canvas
}

// Every v1 module x eye style renders like the v1 renderer. The new PNG is anti-aliased, so
// edge pixels may change; any pixel that changes by more than half the contrast must sit on
// a v1 edge (a neighbor of a v1 pixel of the other color). Interiors never change.
func TestV1StylesKeepTheirLook(t *testing.T) {
	payload := "https://comcode.technowide.software/c/cc-89a12b?src=qr"
	const size = 1024
	logoID := uuid.New()
	clear := image.NewRGBA(image.Rect(0, 0, 16, 16)) // a fully transparent logo shows the backing tile
	for _, module := range []string{"square", "rounded", "dots"} {
		for _, eye := range []string{"square", "rounded", "circle", "logo"} {
			v1 := Style{Foreground: "#1f2937", Background: "#fff7ed", Module: module, Eye: eye, QuietZone: 4}
			var logo image.Image
			if eye == "logo" {
				v1.Eye, v1.LogoProfileUUID, v1.LogoField, logo = "rounded", &logoID, "avatar", clear
			}
			m, err := encode(payload, logo != nil)
			if err != nil {
				t.Fatal(err)
			}
			data, _, err := Render(payload, "png", size, v1, logo)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := png.Decode(bytes.NewReader(data))
			want := legacyRender(m, v1, size)
			half := float64(parseColor(v1.Background).R-parseColor(v1.Foreground).R) / 2
			red := func(x, y int) float64 { return float64(want.RGBAAt(x, y).R) }
			interior := 0
			for y := 1; y < size-1; y++ {
				for x := 1; x < size-1; x++ {
					a, _, _, _ := got.At(x, y).RGBA()
					if math.Abs(float64(a>>8)-red(x, y)) <= half {
						continue
					}
					edge := false
					for _, d := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, 1}, {-1, 1}, {1, -1}} {
						if red(x+d[0], y+d[1]) != red(x, y) {
							edge = true
						}
					}
					if !edge {
						interior++
					}
				}
			}
			if interior > 0 {
				t.Errorf("%s/%s: %d pixels changed away from v1 edges", module, eye, interior)
			}
		}
	}
}
