package qr

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
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
	data, ct, err := Render(payload, "png", size, DefaultStyle(), nil)
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
