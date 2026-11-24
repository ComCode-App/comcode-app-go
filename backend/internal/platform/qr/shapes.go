package qr

// Shape catalogs. IDs are ComCode's own descriptive names; every shape is drawn here from
// plain geometry (rectangles with square, rounded or cut corners, circles and cubic curves).
// Order is the editor's display order. Keep in sync with qr-engine.ts (the fixtures check it).

// BodyShapes are the data-module shapes. Connected shapes look at their orthogonal
// neighbors: only corners with no neighbor on either side are shaped, so runs of modules merge
// into one outline; "-in" shapes also fill the inner (concave) corners between modules.
var BodyShapes = []string{
	"square", "rounded-square", "circle", "small-dot", "mosaic", "spaced-square", "diamond", "star",
	"fluid", "smooth", "chamfer", "chamfer-soft", "rounded-in", "rounded-in-soft", "pointed-in", "pointed-in-soft",
	"pointed", "pointed-soft", "leaf", "rounded-pointed", "chamfer-leaf",
	"capsule-horizontal", "capsule-vertical",
}

// EyeFrameShapes are the 7x7 finder rings.
var EyeFrameShapes = []string{
	"square", "rounded", "circle", "rounded-large", "rounded-outer", "rounded-inner", "leaf",
	"rounded-corner", "rounded-three", "chamfer", "chamfer-leaf", "dotted", "beaded", "concave", "rough",
}

// EyeBallShapes are the 3x3 finder centers.
var EyeBallShapes = []string{
	"square", "rounded", "circle", "rounded-soft", "leaf", "rounded-corner", "rounded-three", "chamfer",
	"chamfer-leaf", "dots-3x3", "capsules-horizontal", "capsules-vertical", "clover", "star", "concave",
	"rough", "cushion", "cross",
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func IsBodyShape(v string) bool     { return contains(BodyShapes, v) }
func IsEyeFrameShape(v string) bool { return contains(EyeFrameShapes, v) }
func IsEyeBallShape(v string) bool  { return contains(EyeBallShapes, v) }

const (
	bodyRect      = iota // independent rectangle: inset + corners
	bodyDot              // independent circle of diameter D
	bodyConnected        // neighbor-aware outer corners + optional concave fill
	bodyCapsuleH
	bodyCapsuleV
	bodyDiamond
	bodyStar
)

type bodySpec struct {
	Kind  int
	Inset float64
	D     float64
	Outer [4]corner // TL, TR, BR, BL
	Inner corner    // concave fill; square means none
}

var bodySpecs = map[string]bodySpec{
	"square":             {Kind: bodyRect},
	"rounded-square":     {Kind: bodyRect, Outer: all4(rnd(.2))}, // v1 "rounded"
	"circle":             {Kind: bodyDot, D: 1},                  // v1 "dots"
	"small-dot":          {Kind: bodyDot, D: .7},
	"mosaic":             {Kind: bodyRect, Inset: .06},
	"spaced-square":      {Kind: bodyRect, Inset: .15, Outer: all4(rnd(.08))},
	"diamond":            {Kind: bodyDiamond},
	"star":               {Kind: bodyStar},
	"fluid":              {Kind: bodyConnected, Outer: all4(rnd(.5))},
	"smooth":             {Kind: bodyConnected, Outer: all4(rnd(.3))}, // rounded square alone, merged in runs
	"chamfer":            {Kind: bodyConnected, Outer: all4(cut(.3))},
	"chamfer-soft":       {Kind: bodyConnected, Outer: all4(cut(.2)), Inner: cut(.2)},
	"rounded-in":         {Kind: bodyConnected, Inner: rnd(.5)},
	"rounded-in-soft":    {Kind: bodyConnected, Outer: all4(rnd(.5)), Inner: rnd(.5)},
	"pointed-in":         {Kind: bodyConnected, Inner: cut(.5)},
	"pointed-in-soft":    {Kind: bodyConnected, Outer: all4(rnd(.25)), Inner: cut(.35)},
	"pointed":            {Kind: bodyConnected, Outer: [4]corner{rnd(.5), rnd(.5), sq, rnd(.5)}},
	"pointed-soft":       {Kind: bodyConnected, Outer: [4]corner{rnd(.5), rnd(.5), rnd(.15), rnd(.5)}},
	"leaf":               {Kind: bodyConnected, Outer: [4]corner{rnd(.5), sq, rnd(.5), sq}},
	"rounded-pointed":    {Kind: bodyConnected, Outer: [4]corner{rnd(.5), cut(.25), rnd(.5), cut(.25)}},
	"chamfer-leaf":       {Kind: bodyConnected, Outer: [4]corner{cut(.5), sq, cut(.5), sq}},
	"capsule-horizontal": {Kind: bodyCapsuleH},
	"capsule-vertical":   {Kind: bodyCapsuleV},
}

// grid answers whether a data module is dark. Finder patterns and logo-cleared modules are
// light for the body, so body shapes never merge into them.
type grid struct {
	n   int
	on  func(x, y int) bool
	off func(x, y int) bool // true where the body may not draw at all (finders, cleared logo area)
}

func inFinder(n, x, y int) bool {
	return (x < 7 && y < 7) || (x >= n-7 && y < 7) || (x < 7 && y >= n-7)
}

func bodyGeometry(g grid, shape string, q float64) []contour {
	spec := bodySpecs[shape]
	var b builder
	for y := 0; y < g.n; y++ {
		for x := 0; x < g.n; x++ {
			if g.off(x, y) {
				continue
			}
			px, py := float64(x)+q, float64(y)+q
			if !g.on(x, y) {
				if spec.Kind == bodyConnected && spec.Inner.Kind != cornerSquare {
					concave(&b, g, x, y, px, py, spec.Inner)
				}
				continue
			}
			nN, nE, nS, nW := g.on(x, y-1), g.on(x+1, y), g.on(x, y+1), g.on(x-1, y)
			switch spec.Kind {
			case bodyRect:
				w := 1 - spec.Inset - spec.Inset
				b.rect(px+spec.Inset, py+spec.Inset, w, w, spec.Outer)
			case bodyDot:
				b.circle(px+.5, py+.5, spec.D/2)
			case bodyConnected:
				var cs [4]corner
				if !nN && !nW {
					cs[0] = spec.Outer[0]
				}
				if !nN && !nE {
					cs[1] = spec.Outer[1]
				}
				if !nS && !nE {
					cs[2] = spec.Outer[2]
				}
				if !nS && !nW {
					cs[3] = spec.Outer[3]
				}
				b.rect(px, py, 1, 1, cs)
			case bodyCapsuleH:
				var cs [4]corner
				if !nW {
					cs[0], cs[3] = rnd(.4), rnd(.4)
				}
				if !nE {
					cs[1], cs[2] = rnd(.4), rnd(.4)
				}
				b.rect(px, py+.1, 1, .8, cs)
			case bodyCapsuleV:
				var cs [4]corner
				if !nN {
					cs[0], cs[1] = rnd(.4), rnd(.4)
				}
				if !nS {
					cs[2], cs[3] = rnd(.4), rnd(.4)
				}
				b.rect(px+.1, py, .8, 1, cs)
			case bodyDiamond:
				b.cornerPoly([]pt{{px + .5, py}, {px + 1, py + .5}, {px + .5, py + 1}, {px, py + .5}}, make([]corner, 4))
			case bodyStar:
				b.cornerPoly([]pt{
					{px + .5, py}, {px + .7, py + .3}, {px + 1, py + .5}, {px + .7, py + .7},
					{px + .5, py + 1}, {px + .3, py + .7}, {px, py + .5}, {px + .3, py + .3},
				}, make([]corner, 8))
			}
		}
	}
	return b.out
}

// concave fills the corners of a light cell that sit inside an L of three dark modules, which
// rounds (or bevels) the inner corners of connected shapes.
func concave(b *builder, g grid, x, y int, px, py float64, c corner) {
	r := c.R
	type fill struct {
		dx, dy, dd  [2]int // the two orthogonal neighbors and the diagonal
		p, toA, toB pt     // corner point and offsets along its two edges (clockwise)
	}
	fills := []fill{
		{[2]int{-1, 0}, [2]int{0, -1}, [2]int{-1, -1}, pt{px, py}, pt{r, 0}, pt{0, r}},
		{[2]int{0, -1}, [2]int{1, 0}, [2]int{1, -1}, pt{px + 1, py}, pt{0, r}, pt{-r, 0}},
		{[2]int{1, 0}, [2]int{0, 1}, [2]int{1, 1}, pt{px + 1, py + 1}, pt{-r, 0}, pt{0, -r}},
		{[2]int{0, 1}, [2]int{-1, 0}, [2]int{-1, 1}, pt{px, py + 1}, pt{0, -r}, pt{r, 0}},
	}
	for _, f := range fills {
		if !g.on(x+f.dx[0], y+f.dx[1]) || !g.on(x+f.dy[0], y+f.dy[1]) || !g.on(x+f.dd[0], y+f.dd[1]) {
			continue
		}
		a, e := add(f.p, f.toA), add(f.p, f.toB)
		b.move(f.p)
		b.line(a)
		if c.Kind == cornerRound {
			b.cubic(add(a, scale(sub(f.p, a), kappa)), add(e, scale(sub(f.p, e), kappa)), e)
		} else {
			b.line(e)
		}
		b.close()
	}
}

// eyeFrame builds a 7x7 finder ring in local coordinates (0..7).
func eyeFrame(shape string) []contour {
	var b builder
	ring := func(o, i [4]corner) {
		b.rect(0, 0, 7, 7, o)
		b.hole(1, 1, 5, 5, i)
	}
	perimeter := func(f func(x, y float64)) {
		for i := 0; i < 7; i++ {
			f(float64(i), 0)
		}
		for j := 1; j < 6; j++ {
			f(6, float64(j))
		}
		for i := 6; i >= 0; i-- {
			f(float64(i), 6)
		}
		for j := 5; j >= 1; j-- {
			f(0, float64(j))
		}
	}
	switch shape {
	case "square":
		ring(all4(sq), all4(sq))
	case "rounded":
		ring(all4(rnd(1)), all4(rnd(1)))
	case "circle":
		ring(all4(rnd(3.5)), all4(rnd(2.5)))
	case "rounded-large":
		ring(all4(rnd(2.5)), all4(rnd(1.5)))
	case "rounded-outer":
		ring(all4(rnd(2)), all4(rnd(.5)))
	case "rounded-inner":
		ring(all4(sq), all4(rnd(1.5)))
	case "leaf":
		ring([4]corner{rnd(2.5), sq, rnd(2.5), sq}, [4]corner{rnd(1.5), sq, rnd(1.5), sq})
	case "rounded-corner":
		ring([4]corner{rnd(2.5), sq, sq, sq}, [4]corner{rnd(1.5), sq, sq, sq})
	case "rounded-three":
		ring([4]corner{rnd(2.5), rnd(2.5), sq, rnd(2.5)}, [4]corner{rnd(1.5), rnd(1.5), sq, rnd(1.5)})
	case "chamfer":
		ring(all4(cut(1.5)), all4(cut(.914)))
	case "chamfer-leaf":
		ring([4]corner{cut(2), sq, cut(2), sq}, [4]corner{cut(1.1), sq, cut(1.1), sq})
	case "dotted":
		perimeter(func(x, y float64) { b.circle(x+.5, y+.5, .55) }) // dots overlap slightly: a closed ring for scanners
	case "beaded":
		perimeter(func(x, y float64) { b.rect(x+.05, y+.05, .9, .9, all4(rnd(.25))) })
	case "concave":
		// Square outline; the hole's sides bow into it (counter-clockwise), so the ring is
		// thicker mid-side while the outer edge stays on the 7-module grid scanners measure.
		t, u := 1+5.0/3, 1+10.0/3
		b.rect(0, 0, 7, 7, all4(sq))
		b.move(pt{1, 1})
		b.cubic(pt{1.3, t}, pt{1.3, u}, pt{1, 6})
		b.cubic(pt{t, 5.7}, pt{u, 5.7}, pt{6, 6})
		b.cubic(pt{5.7, u}, pt{5.7, t}, pt{6, 1})
		b.cubic(pt{u, 1.3}, pt{t, 1.3}, pt{1, 1})
		b.close()
	case "rough":
		b.rect(.2, .2, 6.6, 6.6, all4(sq))
		b.hole(1, 1, 5, 5, all4(sq))
		for i := 1.0; i < 7; i += 2 {
			b.rect(i, 0, 1, .2, all4(sq))
			b.rect(6.8, i, .2, 1, all4(sq))
			b.rect(i, 6.8, 1, .2, all4(sq))
			b.rect(0, i, .2, 1, all4(sq))
		}
	}
	return b.out
}

// eyeBall builds the 3x3 finder center in the same local coordinates (2..5).
func eyeBall(shape string) []contour {
	var b builder
	box := func(cs [4]corner) { b.rect(2, 2, 3, 3, cs) }
	switch shape {
	case "square":
		box(all4(sq))
	case "rounded":
		box(all4(rnd(1)))
	case "circle":
		box(all4(rnd(1.5)))
	case "rounded-soft":
		box(all4(rnd(.5)))
	case "leaf":
		box([4]corner{rnd(1.5), sq, rnd(1.5), sq})
	case "rounded-corner":
		box([4]corner{rnd(1.5), sq, sq, sq})
	case "rounded-three":
		box([4]corner{rnd(1.5), rnd(1.5), sq, rnd(1.5)})
	case "chamfer":
		box(all4(cut(.75)))
	case "chamfer-leaf":
		box([4]corner{cut(.9), sq, cut(.9), sq})
	case "dots-3x3":
		// Dots overlap slightly on the diagonals: scanners check the 1:1:3:1:1 ratio diagonally too.
		for j := 0.0; j < 3; j++ {
			for i := 0.0; i < 3; i++ {
				b.circle(2.6+mul(i, .9), 2.6+mul(j, .9), .66)
			}
		}
	case "capsules-horizontal":
		for j := 0.0; j < 3; j++ {
			b.rect(2, 2+j, 3, 1, all4(rnd(.5)))
		}
	case "capsules-vertical":
		for i := 0.0; i < 3; i++ {
			b.rect(2+i, 2, 1, 3, all4(rnd(.5)))
		}
	case "clover":
		b.circle(2.9, 2.9, .9)
		b.circle(4.1, 2.9, .9)
		b.circle(4.1, 4.1, .9)
		b.circle(2.9, 4.1, .9)
	case "star":
		b.cornerPoly([]pt{
			{3.5, 2}, {4.55, 2.45}, {5, 3.5}, {4.55, 4.55}, {3.5, 5}, {2.45, 4.55}, {2, 3.5}, {2.45, 2.45},
		}, make([]corner, 8))
	case "concave":
		b.move(pt{2, 2})
		b.cubic(pt{3, 2.4}, pt{4, 2.4}, pt{5, 2})
		b.cubic(pt{4.6, 3}, pt{4.6, 4}, pt{5, 5})
		b.cubic(pt{4, 4.6}, pt{3, 4.6}, pt{2, 5})
		b.cubic(pt{2.4, 4}, pt{2.4, 3}, pt{2, 2})
		b.close()
	case "rough":
		b.rect(2.2, 2.2, 2.6, 2.6, all4(sq))
		for _, i := range []float64{2, 4} {
			b.rect(i, 2, 1, .2, all4(sq))
			b.rect(4.8, i, .2, 1, all4(sq))
			b.rect(i, 4.8, 1, .2, all4(sq))
			b.rect(2, i, .2, 1, all4(sq))
		}
	case "cushion":
		// Sides bow out by 0.15 module: corners stay full for diagonal scans.
		b.move(pt{2, 2})
		b.cubic(pt{3, 1.8}, pt{4, 1.8}, pt{5, 2})
		b.cubic(pt{5.2, 3}, pt{5.2, 4}, pt{5, 5})
		b.cubic(pt{4, 5.2}, pt{3, 5.2}, pt{2, 5})
		b.cubic(pt{1.8, 4}, pt{1.8, 3}, pt{2, 2})
		b.close()
	case "cross":
		b.cornerPoly([]pt{
			{2.45, 2}, {4.55, 2}, {4.55, 2.45}, {5, 2.45}, {5, 4.55}, {4.55, 4.55},
			{4.55, 5}, {2.45, 5}, {2.45, 4.55}, {2, 4.55}, {2, 2.45}, {2.45, 2.45},
		}, []corner{rnd(.3), rnd(.3), sq, rnd(.3), rnd(.3), sq, rnd(.3), rnd(.3), sq, rnd(.3), rnd(.3), sq})
	}
	return b.out
}

// placeEye moves a local eye shape to its corner and applies the eye's flips.
func placeEye(cs []contour, ox, oy float64, e EyeStyle) []contour {
	return transform(cs, func(p pt) pt {
		x, y := p.X, p.Y
		if e.FlipH {
			x = 7 - x
		}
		if e.FlipV {
			y = 7 - y
		}
		return pt{x + ox, y + oy}
	}, e.FlipH != e.FlipV)
}
