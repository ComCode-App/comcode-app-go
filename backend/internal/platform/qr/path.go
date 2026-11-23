package qr

import (
	"math"
	"strconv"
	"strings"
)

// The geometry engine is mirrored line by line in
// frontend/src/features/route-qr/lib/qr-engine.ts and both are checked against the golden
// fixtures in testdata/. Keep the arithmetic in the same order in both languages: only
// +, -, *, / and sqrt are used (all correctly rounded in IEEE 754), products that feed a sum
// are rounded explicitly with float64() so the Go compiler cannot fuse them into an FMA, and
// output numbers are rounded to three decimals by fmtNum.

type pt struct{ X, Y float64 }

// seg is a line (Cubic false) or cubic Bézier to P.
type seg struct {
	Cubic  bool
	C1, C2 pt
	P      pt
}

type contour struct {
	Start pt
	Segs  []seg
}

// kappa places cubic control points for a quarter circle: 4/3 * (sqrt(2) - 1).
const kappa = 0.5522847498307936

func mul(a, b float64) float64 { return float64(a * b) }

func add(a, b pt) pt           { return pt{a.X + b.X, a.Y + b.Y} }
func sub(a, b pt) pt           { return pt{a.X - b.X, a.Y - b.Y} }
func scale(a pt, k float64) pt { return pt{mul(a.X, k), mul(a.Y, k)} }

// unit is the direction from a to b.
func unit(a, b pt) pt {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := math.Sqrt(float64(dx*dx) + float64(dy*dy))
	return pt{dx / l, dy / l}
}

type builder struct {
	out []contour
	cur *contour
	at  pt
}

func (b *builder) move(p pt) {
	b.out = append(b.out, contour{Start: p})
	b.cur = &b.out[len(b.out)-1]
	b.at = p
}

func (b *builder) line(p pt) {
	if p == b.at {
		return
	}
	b.cur.Segs = append(b.cur.Segs, seg{P: p})
	b.at = p
}

func (b *builder) cubic(c1, c2, p pt) {
	b.cur.Segs = append(b.cur.Segs, seg{Cubic: true, C1: c1, C2: c2, P: p})
	b.at = p
}

// close drops a trailing straight line back to the start (Z draws it).
func (b *builder) close() {
	if n := len(b.cur.Segs); n > 0 && !b.cur.Segs[n-1].Cubic && b.cur.Segs[n-1].P == b.cur.Start {
		b.cur.Segs = b.cur.Segs[:n-1]
	}
	b.cur = nil
}

// Corner treatments for cornerPoly.
const (
	cornerSquare = iota
	cornerRound
	cornerCut
)

type corner struct {
	Kind int
	R    float64
}

var sq = corner{}

func rnd(r float64) corner { return corner{cornerRound, r} }
func cut(r float64) corner { return corner{cornerCut, r} }
func all4(c corner) [4]corner {
	return [4]corner{c, c, c, c}
}

// cornerPoly adds a closed polygon whose corners are kept square, rounded (cubic fillet of
// radius R) or cut (straight chamfer R along both edges). Clockwise points (screen
// coordinates, y down) give a filled shape; counter-clockwise points cut a hole.
func (b *builder) cornerPoly(pts []pt, cs []corner) {
	n := len(pts)
	dirIn := func(i int) pt { return unit(pts[(i+n-1)%n], pts[i]) }
	dirOut := func(i int) pt { return unit(pts[i], pts[(i+1)%n]) }
	if cs[0].Kind == cornerSquare {
		b.move(pts[0])
	} else {
		b.move(add(pts[0], scale(dirOut(0), cs[0].R)))
	}
	for k := 1; k <= n; k++ {
		i := k % n
		p, c := pts[i], cs[i]
		switch c.Kind {
		case cornerSquare:
			b.line(p)
		case cornerCut:
			b.line(sub(p, scale(dirIn(i), c.R)))
			b.line(add(p, scale(dirOut(i), c.R)))
		case cornerRound:
			din, dout := dirIn(i), dirOut(i)
			a := sub(p, scale(din, c.R))
			e := add(p, scale(dout, c.R))
			b.line(a)
			b.cubic(add(a, scale(din, mul(c.R, kappa))), sub(e, scale(dout, mul(c.R, kappa))), e)
		}
	}
	b.close()
}

func rectPts(x, y, w, h float64) []pt {
	return []pt{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
}

// rect adds a clockwise rectangle with corners [top-left, top-right, bottom-right, bottom-left].
func (b *builder) rect(x, y, w, h float64, cs [4]corner) {
	b.cornerPoly(rectPts(x, y, w, h), cs[:])
}

// hole adds a counter-clockwise rectangle (same corner order as rect).
func (b *builder) hole(x, y, w, h float64, cs [4]corner) {
	p := rectPts(x, y, w, h)
	b.cornerPoly([]pt{p[0], p[3], p[2], p[1]}, []corner{cs[0], cs[3], cs[2], cs[1]})
}

func (b *builder) circle(cx, cy, r float64) {
	b.rect(cx-r, cy-r, r+r, r+r, all4(rnd(r)))
}

// transform maps every point; a mirrored mapping reverses each contour so fills stay
// clockwise and holes counter-clockwise.
func transform(cs []contour, f func(pt) pt, mirrored bool) []contour {
	out := make([]contour, 0, len(cs))
	for _, c := range cs {
		start := f(c.Start)
		segs := make([]seg, len(c.Segs))
		for i, s := range c.Segs {
			segs[i] = seg{Cubic: s.Cubic, C1: f(s.C1), C2: f(s.C2), P: f(s.P)}
		}
		if !mirrored {
			out = append(out, contour{Start: start, Segs: segs})
			continue
		}
		var b builder
		b.move(start)
		if len(segs) > 0 {
			b.line(segs[len(segs)-1].P)
		}
		for i := len(segs) - 1; i >= 0; i-- {
			prev := start
			if i > 0 {
				prev = segs[i-1].P
			}
			if segs[i].Cubic {
				b.cubic(segs[i].C2, segs[i].C1, prev)
			} else {
				b.line(prev)
			}
		}
		b.close()
		out = append(out, b.out...)
	}
	return out
}

// fmtNum rounds half away from zero to three decimals and prints the shortest form
// (no exponent, no trailing zeros, never "-0"), identical to the TypeScript fmtNum.
func fmtNum(v float64) string {
	r := math.Round(v*1000) / 1000
	if r == 0 {
		return "0"
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// pathData prints contours as SVG path data with absolute M/L/C/Z commands.
func pathData(cs []contour) string {
	var sb strings.Builder
	pair := func(p pt) {
		sb.WriteString(fmtNum(p.X))
		sb.WriteByte(' ')
		sb.WriteString(fmtNum(p.Y))
	}
	for _, c := range cs {
		sb.WriteByte('M')
		pair(c.Start)
		for _, s := range c.Segs {
			if s.Cubic {
				sb.WriteByte('C')
				pair(s.C1)
				sb.WriteByte(' ')
				pair(s.C2)
				sb.WriteByte(' ')
			} else {
				sb.WriteByte('L')
			}
			pair(s.P)
		}
		sb.WriteByte('Z')
	}
	return sb.String()
}
