package qr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"testing"

	"github.com/google/uuid"
	"github.com/makiuchi-d/gozxing"
	zxqr "github.com/makiuchi-d/gozxing/qrcode"
)

// Readability: every shape is drawn to PNG and decoded back with a QR reader (gozxing, a ZXing
// port, used only in tests). The covering set renders every frame x ball pair (with and without
// a logo) and every body shape against several eyes, which puts each shape in at least a dozen
// combinations; the full body x frame x ball x logo product (11880 renders) adds no new shape.

var readabilityPayloads = []string{
	"https://comcode.technowide.software/username?src=qr",
	"https://comcode.technowide.software/c/cc-89a12b?src=qr",
}

// testLogo is an opaque, busy image: worst case for the modules it covers.
func testLogo() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 5), B: 160, A: 255})
		}
	}
	return img
}

func decodePNG(t *testing.T, data []byte) (string, error) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	// Transparent output is judged on white, as Validate assumes.
	flat := image.NewRGBA(img.Bounds())
	draw.Draw(flat, flat.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), img, img.Bounds().Min, draw.Over)
	bmp, err := gozxing.NewBinaryBitmapFromImage(flat)
	if err != nil {
		t.Fatal(err)
	}
	res, err := zxqr.NewQRCodeReader().Decode(bmp, map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true})
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

func assertReadable(t *testing.T, name string, s Style, logo image.Image) {
	t.Helper()
	for _, payload := range readabilityPayloads {
		data, _, err := Render(payload, "png", 320, s, logo)
		if err != nil {
			t.Fatalf("%s: render: %v", name, err)
		}
		got, err := decodePNG(t, data)
		if err != nil || got != payload {
			t.Errorf("%s: not readable (%v, %q)", name, err, got)
			return
		}
	}
}

func withLogo(s Style, on bool) Style {
	if on {
		id := uuid.MustParse("00000000-0000-4000-8000-000000000001")
		s.LogoProfileUUID, s.LogoField, s.LogoSize = &id, "avatar", LogoSizeMax
	}
	return s
}

func TestEveryShapeIsReadable(t *testing.T) {
	if testing.Short() {
		t.Skip("readability sweep skipped in -short mode")
	}
	logo := testLogo()
	for fi, frame := range EyeFrameShapes {
		for bi, ball := range EyeBallShapes {
			for _, hasLogo := range []bool{false, true} {
				s := DefaultStyle()
				s.BodyShape = BodyShapes[(fi+bi)%len(BodyShapes)]
				s.EyeFrameShape, s.EyeBallShape = frame, ball
				s = withLogo(s, hasLogo)
				var l image.Image
				if hasLogo {
					l = logo
				}
				assertReadable(t, fmt.Sprintf("%s/%s/%s/logo=%v", s.BodyShape, frame, ball, hasLogo), s, l)
			}
		}
	}
	for i, body := range BodyShapes {
		for j := 0; j < 3; j++ {
			for _, hasLogo := range []bool{false, true} {
				s := DefaultStyle()
				s.BodyShape = body
				s.EyeFrameShape = EyeFrameShapes[(i*3+j)%len(EyeFrameShapes)]
				s.EyeBallShape = EyeBallShapes[(i*3+j)%len(EyeBallShapes)]
				s = withLogo(s, hasLogo)
				var l image.Image
				if hasLogo {
					l = logo
				}
				assertReadable(t, fmt.Sprintf("%s/%s/%s/logo=%v", body, s.EyeFrameShape, s.EyeBallShape, hasLogo), s, l)
			}
		}
	}
}

// Color modes, flips, transparency and logo modes on demanding shapes, all at the limits the
// validator allows (4.5:1 colors, the largest logo).
func TestColorAndLogoModesAreReadable(t *testing.T) {
	logo := testLogo()
	base := DefaultStyle()
	base.BodyShape, base.EyeFrameShape, base.EyeBallShape = "small-dot", "dotted", "dots-3x3"
	cases := map[string]func(*Style){
		"linear gradient": func(s *Style) {
			s.FillMode, s.GradientStart, s.GradientEnd, s.GradientAngle, s.GradientOnEyes = FillLinear, "#000000", "#595959", 135, true
		},
		"radial gradient": func(s *Style) {
			s.FillMode, s.GradientStart, s.GradientEnd = FillRadial, "#1e3a8a", "#595959"
		},
		"eye colors and flips": func(s *Style) {
			s.Eyes = [3]EyeStyle{{FrameColor: "#595959", BallColor: "#7c2d12"}, {FlipH: true, FrameColor: "#14532d"}, {FlipV: true, FlipH: true, BallColor: "#4c1d95"}}
		},
		"transparent": func(s *Style) { s.TransparentBackground = true; s.Foreground = "#595959" },
		"tinted background": func(s *Style) {
			s.Background, s.Foreground = "#fff7ed", "#5b3a1a"
		},
		"clean logo":              func(s *Style) { *s = withLogo(*s, true) },
		"clean logo transparent":  func(s *Style) { *s = withLogo(*s, true); s.TransparentBackground = true },
		"overlay logo":            func(s *Style) { *s = withLogo(*s, true); s.LogoMode = LogoOverlay },
		"overlay logo with flips": func(s *Style) { *s = withLogo(*s, true); s.LogoMode = LogoOverlay; s.Eyes[1].FlipV = true },
	}
	for name, change := range cases {
		s := base
		change(&s)
		if err := s.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var l image.Image
		if s.LogoProfileUUID != nil {
			l = logo
		}
		assertReadable(t, name, s, l)
	}
}
