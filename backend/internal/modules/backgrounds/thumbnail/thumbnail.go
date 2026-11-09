// Package thumbnail draws the static stand-in image of a Background Studio design (TEC-599).
//
// Public pages show it on devices that cannot or should not run WebGL (TEC-600), and pickers
// use it as a preview tile. Shaders cannot run on the server, so a shader is drawn from its
// palette (the same colors the client-side fallback in lib/shader-fallback.ts derives) as a soft
// mesh blend; solid and gradient designs are drawn exactly. Videos have no thumbnail: their
// poster is the stand-in.
package thumbnail

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Width and Height are the thumbnail size: portrait, since public pages are mostly opened on
// phones. Smooth gradients scale up without visible loss.
const (
	Width  = 360
	Height = 640
)

// ContentType of the rendered image.
const ContentType = "image/jpeg"

type rgb struct{ r, g, b float64 }

var defaultBase = rgb{0x0f, 0x17, 0x2a} // #0f172a, the client fallback's default

// Render draws the document. ok is false for kinds without a thumbnail (video, unknown).
func Render(raw json.RawMessage) (img []byte, ok bool, err error) {
	var doc struct {
		Kind   string         `json:"kind"`
		Params map[string]any `json:"params"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, false, err
	}
	var canvas *image.RGBA
	switch doc.Kind {
	case "solid":
		c, ok := parseColor(str(doc.Params["color"]))
		if !ok {
			c = defaultBase
		}
		canvas = fill(c)
	case "gradient":
		canvas = gradient(doc.Params)
	case "shader":
		canvas = mesh(doc.Params)
	default:
		return nil, false, nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: 82}); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func colorList(v any) []rgb {
	items, _ := v.([]any)
	out := make([]rgb, 0, len(items))
	for _, item := range items {
		if c, ok := parseColor(str(item)); ok {
			out = append(out, c)
		}
	}
	return out
}

func newCanvas() *image.RGBA { return image.NewRGBA(image.Rect(0, 0, Width, Height)) }

func set(img *image.RGBA, x, y int, c rgb) {
	img.SetRGBA(x, y, color.RGBA{R: clamp8(c.r), G: clamp8(c.g), B: clamp8(c.b), A: 255})
}

func fill(c rgb) *image.RGBA {
	img := newCanvas()
	for y := range Height {
		for x := range Width {
			set(img, x, y, c)
		}
	}
	return img
}

// gradient mirrors the CSS the public page uses: linear-gradient(<angle>deg, ...) or
// radial-gradient(circle at center, ...), with evenly spaced stops.
func gradient(params map[string]any) *image.RGBA {
	colors := colorList(params["colors"])
	if len(colors) < 2 {
		colors = []rgb{{0xe0, 0xea, 0xff}, {0x24, 0x1d, 0x9a}} // the page's default stops
	}
	angle := 135.0
	if n, ok := params["angle"].(float64); ok && !math.IsNaN(n) && !math.IsInf(n, 0) {
		angle = n
	}
	radial := str(params["type"]) == "radial"
	img := newCanvas()
	cx, cy := float64(Width)/2, float64(Height)/2
	rad := angle * math.Pi / 180
	dx, dy := math.Sin(rad), -math.Cos(rad) // CSS: 0deg points up, 90deg right
	length := math.Abs(Width*dx) + math.Abs(Height*dy)
	corner := math.Hypot(cx, cy) // farthest-corner circle
	for y := range Height {
		for x := range Width {
			px, py := float64(x)+0.5-cx, float64(y)+0.5-cy
			var t float64
			if radial {
				t = math.Hypot(px, py) / corner
			} else {
				t = (px*dx+py*dy)/length + 0.5
			}
			set(img, x, y, stops(colors, t))
		}
	}
	return img
}

func stops(colors []rgb, t float64) rgb {
	t = math.Max(0, math.Min(1, t))
	pos := t * float64(len(colors)-1)
	i := int(pos)
	if i >= len(colors)-1 {
		return colors[len(colors)-1]
	}
	return mix(colors[i], colors[i+1], pos-float64(i))
}

// palette mirrors shaderPalette in lib/shader-fallback.ts: colorBack is the base, then the
// colors list, the well-known accent keys, then any other color* param (sorted for stability).
func palette(params map[string]any) (back *rgb, colors []rgb) {
	if c, ok := parseColor(str(params["colorBack"])); ok {
		back = &c
	}
	seen := map[rgb]bool{}
	push := func(c rgb, ok bool) {
		if ok && !seen[c] {
			seen[c] = true
			colors = append(colors, c)
		}
	}
	for _, c := range colorList(params["colors"]) {
		push(c, true)
	}
	for _, key := range []string{"colorFront", "colorInner", "colorTint", "borderColor"} {
		push(parseColor(str(params[key])))
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		if strings.HasPrefix(strings.ToLower(key), "color") && key != "colorBack" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		push(parseColor(str(params[key])))
	}
	return back, colors
}

// Blob centres, in the same order as the client fallback.
var spots = [][2]float64{{0.2, 0.2}, {0.8, 0.25}, {0.75, 0.8}, {0.2, 0.75}, {0.5, 0.5}}

// mesh blends the palette as soft gaussian blobs over the base color, which reads like the
// mesh / grain gradients most designs use.
func mesh(params map[string]any) *image.RGBA {
	back, colors := palette(params)
	base := defaultBase
	switch {
	case back != nil:
		base = *back
	case len(colors) > 0:
		base = colors[0]
	}
	if len(colors) == 0 {
		return fill(base)
	}
	if len(colors) > len(spots) {
		colors = colors[:len(spots)]
	}
	centres := spots
	if len(colors) == 1 {
		centres = [][2]float64{{0.5, 0.4}}
	}
	baseWeight := 0.02 // without a back color the palette covers the canvas
	if back != nil {
		baseWeight = 0.15
	}
	const sigma2 = 2 * 0.3 * 0.3
	aspect := float64(Height) / float64(Width)
	img := newCanvas()
	for y := range Height {
		for x := range Width {
			u, v := (float64(x)+0.5)/Width, (float64(y)+0.5)/Height
			sum := baseWeight
			acc := rgb{base.r * baseWeight, base.g * baseWeight, base.b * baseWeight}
			for i, c := range colors {
				du, dv := u-centres[i][0], (v-centres[i][1])*aspect/2
				w := math.Exp(-(du*du + dv*dv) / sigma2)
				sum += w
				acc.r += c.r * w
				acc.g += c.g * w
				acc.b += c.b * w
			}
			set(img, x, y, rgb{acc.r / sum, acc.g / sum, acc.b / sum})
		}
	}
	return img
}

func mix(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

func clamp8(v float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(v))))
}

var (
	hexColor  = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	funcColor = regexp.MustCompile(`^(rgba?|hsla?)\(([^()]*)\)$`)
)

// parseColor reads #hex, rgb()/rgba() and hsl()/hsla(). Alpha is ignored (the thumbnail is
// opaque); other notations (names, oklch, ...) are skipped like unknown values.
func parseColor(s string) (rgb, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if hexColor.MatchString(s) {
		h := s[1:]
		if len(h) <= 4 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		n, err := strconv.ParseUint(h[:6], 16, 32)
		if err != nil {
			return rgb{}, false
		}
		return rgb{float64(n >> 16 & 0xff), float64(n >> 8 & 0xff), float64(n & 0xff)}, true
	}
	m := funcColor.FindStringSubmatch(s)
	if m == nil {
		return rgb{}, false
	}
	parts := strings.FieldsFunc(m[2], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
	if len(parts) < 3 {
		return rgb{}, false
	}
	if strings.HasPrefix(m[1], "rgb") {
		var c [3]float64
		for i := range 3 {
			v, ok := number(parts[i], 255)
			if !ok {
				return rgb{}, false
			}
			c[i] = v
		}
		return rgb{c[0], c[1], c[2]}, true
	}
	h, ok1 := number(strings.TrimSuffix(parts[0], "deg"), 360)
	sat, ok2 := number(parts[1], 1)
	light, ok3 := number(parts[2], 1)
	if !ok1 || !ok2 || !ok3 {
		return rgb{}, false
	}
	return hsl(h, sat, light), true
}

// number parses a plain number or a percentage of scale. Plain numbers of the 0..1 scale
// (hsl saturation/lightness) are read as percentages, as CSS writes them.
func number(s string, scale float64) (float64, bool) {
	if p, ok := strings.CutSuffix(s, "%"); ok {
		v, err := strconv.ParseFloat(p, 64)
		return v / 100 * scale, err == nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	if scale == 1 && v > 1 {
		v /= 100
	}
	return v, true
}

func hsl(h, s, l float64) rgb {
	h = math.Mod(math.Mod(h, 360)+360, 360) / 360
	s, l = math.Max(0, math.Min(1, s)), math.Max(0, math.Min(1, l))
	if s == 0 {
		return rgb{l * 255, l * 255, l * 255}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		default:
			return p
		}
	}
	return rgb{hue(h+1.0/3) * 255, hue(h) * 255, hue(h-1.0/3) * 255}
}
