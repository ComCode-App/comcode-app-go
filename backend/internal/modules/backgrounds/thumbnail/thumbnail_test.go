package thumbnail

import (
	"bytes"
	"encoding/json"
	"image/jpeg"
	"math"
	"testing"
)

func decode(t *testing.T, raw string) (img []byte, ok bool) {
	t.Helper()
	img, ok, err := Render(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("Render(%s): %v", raw, err)
	}
	return img, ok
}

func pixel(t *testing.T, img []byte, x, y int) rgb {
	t.Helper()
	m, err := jpeg.Decode(bytes.NewReader(img))
	if err != nil {
		t.Fatalf("decode jpeg: %v", err)
	}
	if b := m.Bounds(); b.Dx() != Width || b.Dy() != Height {
		t.Fatalf("size %dx%d, want %dx%d", b.Dx(), b.Dy(), Width, Height)
	}
	r, g, b, _ := m.At(x, y).RGBA()
	return rgb{float64(r >> 8), float64(g >> 8), float64(b >> 8)}
}

func near(a, b rgb) bool {
	return math.Abs(a.r-b.r) < 12 && math.Abs(a.g-b.g) < 12 && math.Abs(a.b-b.b) < 12
}

func TestRenderKinds(t *testing.T) {
	img, ok := decode(t, `{"kind":"solid","params":{"color":"#ff0000"}}`)
	if !ok || !near(pixel(t, img, 10, 10), rgb{255, 0, 0}) {
		t.Fatalf("solid: ok=%v", ok)
	}

	// 180deg runs top to bottom: white at the top, black at the bottom.
	img, ok = decode(t, `{"kind":"gradient","params":{"type":"linear","angle":180,"colors":["#ffffff","#000000"]}}`)
	if !ok {
		t.Fatal("gradient: no thumbnail")
	}
	if top, bottom := pixel(t, img, Width/2, 2), pixel(t, img, Width/2, Height-3); !near(top, rgb{255, 255, 255}) || !near(bottom, rgb{0, 0, 0}) {
		t.Fatalf("gradient: top %v bottom %v", top, bottom)
	}

	// A shader is drawn from its palette: the first color dominates its blob, the back color
	// shows where no blob reaches.
	img, ok = decode(t, `{"kind":"shader","shaderType":"mesh-gradient","params":{"colors":["rgb(0, 128, 255)"],"colorBack":"hsl(0, 0%, 0%)"}}`)
	if !ok {
		t.Fatal("shader: no thumbnail")
	}
	if c := pixel(t, img, Width/2, int(0.4*Height)); c.b < 200 || c.r > 30 {
		t.Fatalf("shader blob centre %v", c)
	}

	if _, ok := decode(t, `{"kind":"video","params":{"src":"https://example.com/a.mp4"}}`); ok {
		t.Fatal("video must not get a thumbnail")
	}
}

func TestParseColor(t *testing.T) {
	cases := map[string]rgb{
		"#abc":                  {0xaa, 0xbb, 0xcc},
		"#AABBCCDD":             {0xaa, 0xbb, 0xcc},
		"rgba(10, 20, 30, 0.5)": {10, 20, 30},
		"rgb(100% 0% 0%)":       {255, 0, 0},
		"hsl(120, 100%, 50%)":   {0, 255, 0},
	}
	for in, want := range cases {
		got, ok := parseColor(in)
		if !ok || !near(got, want) {
			t.Errorf("parseColor(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, bad := range []string{"", "red", "oklch(0.7 0.1 200)", "#12", "rgb(1,2)"} {
		if _, ok := parseColor(bad); ok {
			t.Errorf("parseColor(%q) accepted", bad)
		}
	}
}
