/**
 * Route QR design engine (TEC-1269): the TypeScript twin of backend/internal/platform/qr
 * (style.go, path.go, shapes.go, render.go). Given a module matrix and a style it returns the
 * same SVG, byte for byte, as the Go renderer. Both sides are pinned by the shared golden
 * fixtures in backend/internal/platform/qr/testdata (see qr-engine.test.mjs).
 *
 * Keep the arithmetic in the same order as the Go code: only + - * / and sqrt are used, and
 * output numbers go through fmtNum (three decimals, half away from zero).
 *
 * No imports: node --test loads this file directly with type stripping.
 */

export type QrEye = {
  flip_h: boolean;
  flip_v: boolean;
  frame_color: string;
  ball_color: string;
};

export type QrFillMode = "solid" | "linear" | "radial";
export type QrLogoMode = "clean" | "overlay";

/** Mirrors qr.Style (RouteQRStyle in the OpenAPI schema). */
export type QrStyle = {
  foreground: string;
  background: string;
  transparent_background?: boolean;
  module?: string;
  eye?: string;
  body_shape?: string;
  eye_frame_shape?: string;
  eye_ball_shape?: string;
  eyes?: QrEye[];
  fill_mode?: string;
  gradient_start?: string;
  gradient_end?: string;
  gradient_angle?: number;
  gradient_on_eyes?: boolean;
  quiet_zone: number;
  logo_profile_uuid?: string | null;
  logo_field?: string;
  logo_size?: number;
  logo_mode?: string;
};

/** A style with every v2 field present (the result of normalizeQrStyle). */
export type QrStyleFull = Required<Omit<QrStyle, "logo_profile_uuid">> & {
  logo_profile_uuid: string | null;
};

export const MIN_CONTRAST = 4.5;
export const QUIET_ZONE_MIN = 4;
export const QUIET_ZONE_MAX = 16;
export const LOGO_SIZE_MIN = 10;
export const LOGO_SIZE_MAX = 30;
export const LOGO_SIZE_DEFAULT = 15;

export const BODY_SHAPES = [
  "square",
  "rounded-square",
  "circle",
  "small-dot",
  "mosaic",
  "spaced-square",
  "diamond",
  "star",
  "fluid",
  "chamfer",
  "chamfer-soft",
  "rounded-in",
  "rounded-in-soft",
  "pointed-in",
  "pointed-in-soft",
  "pointed",
  "pointed-soft",
  "leaf",
  "rounded-pointed",
  "chamfer-leaf",
  "capsule-horizontal",
  "capsule-vertical",
] as const;

export const EYE_FRAME_SHAPES = [
  "square",
  "rounded",
  "circle",
  "rounded-large",
  "rounded-outer",
  "rounded-inner",
  "leaf",
  "rounded-corner",
  "rounded-three",
  "chamfer",
  "chamfer-leaf",
  "dotted",
  "beaded",
  "concave",
  "rough",
] as const;

export const EYE_BALL_SHAPES = [
  "square",
  "rounded",
  "circle",
  "rounded-soft",
  "leaf",
  "rounded-corner",
  "rounded-three",
  "chamfer",
  "chamfer-leaf",
  "dots-3x3",
  "capsules-horizontal",
  "capsules-vertical",
  "clover",
  "star",
  "concave",
  "rough",
  "cushion",
  "cross",
] as const;

export type BodyShape = (typeof BODY_SHAPES)[number];
export type EyeFrameShape = (typeof EYE_FRAME_SHAPES)[number];
export type EyeBallShape = (typeof EYE_BALL_SHAPES)[number];

// ---------------------------------------------------------------------------------------------
// Style normalization and validation (style.go)

const HEX = /^#[0-9a-fA-F]{6}$/;

export function isHexColor(value: string): boolean {
  return HEX.test(value);
}

const LEGACY_MODULE_BODY: Record<string, string> = {
  square: "square",
  rounded: "rounded-square",
  dots: "circle",
};
const LEGACY_EYE_FRAME: Record<string, string> = {
  square: "square",
  rounded: "rounded",
  circle: "circle",
};
const LEGACY_EYE_BALL = LEGACY_EYE_FRAME;

function legacyModuleFor(body: string): string {
  for (const [k, v] of Object.entries(LEGACY_MODULE_BODY))
    if (v === body) return k;
  return "square";
}

function legacyEyeFor(frame: string, ball: string): string {
  for (const [k, v] of Object.entries(LEGACY_EYE_FRAME))
    if (v === frame && LEGACY_EYE_BALL[k] === ball) return k;
  return "square";
}

const lowerHex = (v: string) => (HEX.test(v) ? v.toLowerCase() : v);

export const emptyEye = (): QrEye => ({
  flip_h: false,
  flip_v: false,
  frame_color: "",
  ball_color: "",
});

/** Same as qr.Style.Normalize: fills v2 fields from v1 (and v1 from v2) plus defaults. */
export function normalizeQrStyle(input: QrStyle): QrStyleFull {
  const s = input;
  // An omitted v1 field means its default (square), as for a v2-only client.
  const legacyModule = s.module || "square";
  const legacyEye = s.eye || "square";
  const body_shape = s.body_shape || LEGACY_MODULE_BODY[legacyModule] || "";
  const legacyModuleValue = s.module || legacyModuleFor(body_shape);
  const eye_frame_shape =
    s.eye_frame_shape || LEGACY_EYE_FRAME[legacyEye] || "";
  const eye_ball_shape = s.eye_ball_shape || LEGACY_EYE_BALL[legacyEye] || "";
  const eye = s.eye || legacyEyeFor(eye_frame_shape, eye_ball_shape);
  const foreground = lowerHex(s.foreground);
  const eyes = [0, 1, 2].map((i) => {
    const e = s.eyes?.[i] ?? emptyEye();
    return {
      flip_h: Boolean(e.flip_h),
      flip_v: Boolean(e.flip_v),
      frame_color: lowerHex(e.frame_color ?? ""),
      ball_color: lowerHex(e.ball_color ?? ""),
    };
  });
  return {
    foreground,
    background: lowerHex(s.background),
    transparent_background: Boolean(s.transparent_background),
    module: legacyModuleValue,
    eye,
    body_shape,
    eye_frame_shape,
    eye_ball_shape,
    eyes,
    fill_mode: s.fill_mode || "solid",
    gradient_start: lowerHex(s.gradient_start || s.foreground),
    gradient_end: lowerHex(s.gradient_end || s.foreground),
    gradient_angle: s.gradient_angle ?? 0,
    gradient_on_eyes: Boolean(s.gradient_on_eyes),
    quiet_zone: s.quiet_zone,
    logo_profile_uuid: s.logo_profile_uuid || null,
    logo_field: s.logo_field ?? "",
    logo_size: s.logo_size || LOGO_SIZE_DEFAULT,
    logo_mode: s.logo_mode || "clean",
  };
}

function luminance(hex: string): number {
  const v = Number.parseInt(hex.slice(1), 16);
  const linear = (c: number) => {
    const x = c / 255;
    return x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
  };
  return (
    0.2126 * linear((v >> 16) & 255) +
    0.7152 * linear((v >> 8) & 255) +
    0.0722 * linear(v & 255)
  );
}

/** Directional ratio (background over dark); below 1 means light-on-dark ("inverted"). */
export function qrContrast(dark: string, background: string): number | null {
  if (!isHexColor(dark) || !isHexColor(background)) return null;
  return (luminance(background) + 0.05) / (luminance(dark) + 0.05);
}

export type QrColorRole =
  "foreground" | "gradient_start" | "gradient_end" | "eye_frame" | "eye_ball";

export type QrStyleProblem =
  | { kind: "color_format"; role: QrColorRole | "background"; eye?: number }
  | {
      kind: "inverted" | "contrast";
      role: QrColorRole;
      eye?: number;
      ratio: number;
    }
  | { kind: "shape" | "quiet_zone" | "logo" | "gradient" };

/** The dark colors a style actually checks, with their role (mirrors Validate). */
export function darkColors(
  s: QrStyleFull,
): { role: QrColorRole; color: string; eye?: number }[] {
  const out: { role: QrColorRole; color: string; eye?: number }[] = [
    { role: "foreground", color: s.foreground },
  ];
  if (s.fill_mode !== "solid") {
    out.push({ role: "gradient_start", color: s.gradient_start });
    out.push({ role: "gradient_end", color: s.gradient_end });
  }
  s.eyes.forEach((e, eye) => {
    if (e.frame_color)
      out.push({ role: "eye_frame", color: e.frame_color, eye });
    if (e.ball_color) out.push({ role: "eye_ball", color: e.ball_color, eye });
  });
  return out;
}

/** Every rule qr.Style.Validate enforces; an empty list means the API accepts the style. */
export function qrStyleProblems(input: QrStyle): QrStyleProblem[] {
  const s = normalizeQrStyle(input);
  const problems: QrStyleProblem[] = [];
  if (!isHexColor(s.background))
    problems.push({ kind: "color_format", role: "background" });
  const bg = s.transparent_background ? "#ffffff" : s.background;
  for (const c of darkColors(s)) {
    const ratio = qrContrast(c.color, bg);
    if (ratio === null) {
      problems.push({ kind: "color_format", role: c.role, eye: c.eye });
    } else if (isHexColor(s.background) && ratio < MIN_CONTRAST) {
      problems.push({
        kind: ratio < 1 ? "inverted" : "contrast",
        role: c.role,
        eye: c.eye,
        ratio,
      });
    }
  }
  if (
    !["solid", "linear", "radial"].includes(s.fill_mode) ||
    !isHexColor(s.gradient_start) ||
    !isHexColor(s.gradient_end) ||
    !Number.isInteger(s.gradient_angle) ||
    s.gradient_angle < 0 ||
    s.gradient_angle > 359
  )
    problems.push({ kind: "gradient" });
  if (
    !(s.module in LEGACY_MODULE_BODY) ||
    !(s.eye in LEGACY_EYE_FRAME) ||
    !(BODY_SHAPES as readonly string[]).includes(s.body_shape) ||
    !(EYE_FRAME_SHAPES as readonly string[]).includes(s.eye_frame_shape) ||
    !(EYE_BALL_SHAPES as readonly string[]).includes(s.eye_ball_shape)
  )
    problems.push({ kind: "shape" });
  if (
    !Number.isInteger(s.quiet_zone) ||
    s.quiet_zone < QUIET_ZONE_MIN ||
    s.quiet_zone > QUIET_ZONE_MAX
  )
    problems.push({ kind: "quiet_zone" });
  const logoOk =
    Number.isInteger(s.logo_size) &&
    s.logo_size >= LOGO_SIZE_MIN &&
    s.logo_size <= LOGO_SIZE_MAX &&
    (s.logo_mode === "clean" || s.logo_mode === "overlay") &&
    (s.logo_profile_uuid
      ? s.logo_field === "avatar" || s.logo_field === "cover"
      : s.logo_field === "");
  if (!logoOk) problems.push({ kind: "logo" });
  return problems;
}

// ---------------------------------------------------------------------------------------------
// Path building (path.go)

type Pt = { x: number; y: number };
type Seg = { cubic: boolean; c1: Pt; c2: Pt; p: Pt };
type Contour = { start: Pt; segs: Seg[] };

const KAPPA = 0.5522847498307936;
const ZERO: Pt = { x: 0, y: 0 };

const pt = (x: number, y: number): Pt => ({ x, y });
const add = (a: Pt, b: Pt): Pt => pt(a.x + b.x, a.y + b.y);
const sub = (a: Pt, b: Pt): Pt => pt(a.x - b.x, a.y - b.y);
const scale = (a: Pt, k: number): Pt => pt(a.x * k, a.y * k);
const same = (a: Pt, b: Pt) => a.x === b.x && a.y === b.y;

function unit(a: Pt, b: Pt): Pt {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const l = Math.sqrt(dx * dx + dy * dy);
  return pt(dx / l, dy / l);
}

const SQUARE = 0;
const ROUND = 1;
const CUT = 2;
type Corner = { kind: number; r: number };
const sq: Corner = { kind: SQUARE, r: 0 };
const rnd = (r: number): Corner => ({ kind: ROUND, r });
const cut = (r: number): Corner => ({ kind: CUT, r });
const all4 = (c: Corner): Corner[] => [c, c, c, c];

class Builder {
  out: Contour[] = [];
  private cur: Contour | null = null;
  private at: Pt = ZERO;

  move(p: Pt) {
    this.cur = { start: p, segs: [] };
    this.out.push(this.cur);
    this.at = p;
  }

  line(p: Pt) {
    if (same(p, this.at)) return;
    this.cur!.segs.push({ cubic: false, c1: ZERO, c2: ZERO, p });
    this.at = p;
  }

  cubic(c1: Pt, c2: Pt, p: Pt) {
    this.cur!.segs.push({ cubic: true, c1, c2, p });
    this.at = p;
  }

  close() {
    const segs = this.cur!.segs;
    const last = segs[segs.length - 1];
    if (last && !last.cubic && same(last.p, this.cur!.start)) segs.pop();
    this.cur = null;
  }

  cornerPoly(pts: Pt[], cs: Corner[]) {
    const n = pts.length;
    const dirIn = (i: number) => unit(pts[(i + n - 1) % n], pts[i]);
    const dirOut = (i: number) => unit(pts[i], pts[(i + 1) % n]);
    if (cs[0].kind === SQUARE) this.move(pts[0]);
    else this.move(add(pts[0], scale(dirOut(0), cs[0].r)));
    for (let k = 1; k <= n; k++) {
      const i = k % n;
      const p = pts[i];
      const c = cs[i];
      if (c.kind === SQUARE) {
        this.line(p);
      } else if (c.kind === CUT) {
        this.line(sub(p, scale(dirIn(i), c.r)));
        this.line(add(p, scale(dirOut(i), c.r)));
      } else {
        const din = dirIn(i);
        const dout = dirOut(i);
        const a = sub(p, scale(din, c.r));
        const e = add(p, scale(dout, c.r));
        this.line(a);
        this.cubic(
          add(a, scale(din, c.r * KAPPA)),
          sub(e, scale(dout, c.r * KAPPA)),
          e,
        );
      }
    }
    this.close();
  }

  rect(x: number, y: number, w: number, h: number, cs: Corner[]) {
    this.cornerPoly(rectPts(x, y, w, h), cs);
  }

  hole(x: number, y: number, w: number, h: number, cs: Corner[]) {
    const p = rectPts(x, y, w, h);
    this.cornerPoly([p[0], p[3], p[2], p[1]], [cs[0], cs[3], cs[2], cs[1]]);
  }

  circle(cx: number, cy: number, r: number) {
    this.rect(cx - r, cy - r, r + r, r + r, all4(rnd(r)));
  }
}

function rectPts(x: number, y: number, w: number, h: number): Pt[] {
  return [pt(x, y), pt(x + w, y), pt(x + w, y + h), pt(x, y + h)];
}

function transform(
  cs: Contour[],
  f: (p: Pt) => Pt,
  mirrored: boolean,
): Contour[] {
  const out: Contour[] = [];
  for (const c of cs) {
    const start = f(c.start);
    const segs = c.segs.map((s) => ({
      cubic: s.cubic,
      c1: f(s.c1),
      c2: f(s.c2),
      p: f(s.p),
    }));
    if (!mirrored) {
      out.push({ start, segs });
      continue;
    }
    const b = new Builder();
    b.move(start);
    if (segs.length > 0) b.line(segs[segs.length - 1].p);
    for (let i = segs.length - 1; i >= 0; i--) {
      const prev = i > 0 ? segs[i - 1].p : start;
      if (segs[i].cubic) b.cubic(segs[i].c2, segs[i].c1, prev);
      else b.line(prev);
    }
    b.close();
    out.push(...b.out);
  }
  return out;
}

/** Three decimals, half away from zero, shortest form, never "-0" (Go fmtNum). */
export function fmtNum(v: number): string {
  const r = (Math.sign(v) * Math.round(Math.abs(v) * 1000)) / 1000;
  if (r === 0) return "0";
  return String(r);
}

function pathData(cs: Contour[]): string {
  const parts: string[] = [];
  const pair = (p: Pt) => fmtNum(p.x) + " " + fmtNum(p.y);
  for (const c of cs) {
    parts.push("M" + pair(c.start));
    for (const s of c.segs) {
      if (s.cubic)
        parts.push("C" + pair(s.c1) + " " + pair(s.c2) + " " + pair(s.p));
      else parts.push("L" + pair(s.p));
    }
    parts.push("Z");
  }
  return parts.join("");
}

// ---------------------------------------------------------------------------------------------
// Shapes (shapes.go)

const RECT = 0;
const DOT = 1;
const CONNECTED = 2;
const CAPSULE_H = 3;
const CAPSULE_V = 4;
const DIAMOND = 5;
const STAR = 6;

type BodySpec = {
  kind: number;
  inset?: number;
  d?: number;
  outer?: Corner[];
  inner?: Corner;
};

const BODY_SPECS: Record<string, BodySpec> = {
  square: { kind: RECT },
  "rounded-square": { kind: RECT, outer: all4(rnd(0.2)) },
  circle: { kind: DOT, d: 1 },
  "small-dot": { kind: DOT, d: 0.7 },
  mosaic: { kind: RECT, inset: 0.06 },
  "spaced-square": { kind: RECT, inset: 0.15, outer: all4(rnd(0.08)) },
  diamond: { kind: DIAMOND },
  star: { kind: STAR },
  fluid: { kind: CONNECTED, outer: all4(rnd(0.5)) },
  chamfer: { kind: CONNECTED, outer: all4(cut(0.3)) },
  "chamfer-soft": { kind: CONNECTED, outer: all4(cut(0.2)), inner: cut(0.2) },
  "rounded-in": { kind: CONNECTED, inner: rnd(0.5) },
  "rounded-in-soft": {
    kind: CONNECTED,
    outer: all4(rnd(0.5)),
    inner: rnd(0.5),
  },
  "pointed-in": { kind: CONNECTED, inner: cut(0.5) },
  "pointed-in-soft": {
    kind: CONNECTED,
    outer: all4(rnd(0.25)),
    inner: cut(0.35),
  },
  pointed: { kind: CONNECTED, outer: [rnd(0.5), rnd(0.5), sq, rnd(0.5)] },
  "pointed-soft": {
    kind: CONNECTED,
    outer: [rnd(0.5), rnd(0.5), rnd(0.15), rnd(0.5)],
  },
  leaf: { kind: CONNECTED, outer: [rnd(0.5), sq, rnd(0.5), sq] },
  "rounded-pointed": {
    kind: CONNECTED,
    outer: [rnd(0.5), cut(0.25), rnd(0.5), cut(0.25)],
  },
  "chamfer-leaf": { kind: CONNECTED, outer: [cut(0.5), sq, cut(0.5), sq] },
  "capsule-horizontal": { kind: CAPSULE_H },
  "capsule-vertical": { kind: CAPSULE_V },
};

type Grid = {
  n: number;
  on: (x: number, y: number) => boolean;
  off: (x: number, y: number) => boolean;
};

function inFinder(n: number, x: number, y: number): boolean {
  return (x < 7 && y < 7) || (x >= n - 7 && y < 7) || (x < 7 && y >= n - 7);
}

function bodyGeometry(g: Grid, shape: string, q: number): Contour[] {
  const spec = BODY_SPECS[shape];
  const inset = spec.inset ?? 0;
  const outer = spec.outer ?? all4(sq);
  const inner = spec.inner ?? sq;
  const b = new Builder();
  for (let y = 0; y < g.n; y++) {
    for (let x = 0; x < g.n; x++) {
      if (g.off(x, y)) continue;
      const px = x + q;
      const py = y + q;
      if (!g.on(x, y)) {
        if (spec.kind === CONNECTED && inner.kind !== SQUARE)
          concave(b, g, x, y, px, py, inner);
        continue;
      }
      const nN = g.on(x, y - 1);
      const nE = g.on(x + 1, y);
      const nS = g.on(x, y + 1);
      const nW = g.on(x - 1, y);
      switch (spec.kind) {
        case RECT: {
          const w = 1 - inset - inset;
          b.rect(px + inset, py + inset, w, w, outer);
          break;
        }
        case DOT:
          b.circle(px + 0.5, py + 0.5, spec.d! / 2);
          break;
        case CONNECTED:
          b.rect(px, py, 1, 1, [
            !nN && !nW ? outer[0] : sq,
            !nN && !nE ? outer[1] : sq,
            !nS && !nE ? outer[2] : sq,
            !nS && !nW ? outer[3] : sq,
          ]);
          break;
        case CAPSULE_H: {
          const l = nW ? sq : rnd(0.4);
          const r = nE ? sq : rnd(0.4);
          b.rect(px, py + 0.1, 1, 0.8, [l, r, r, l]);
          break;
        }
        case CAPSULE_V: {
          const t = nN ? sq : rnd(0.4);
          const d = nS ? sq : rnd(0.4);
          b.rect(px + 0.1, py, 0.8, 1, [t, t, d, d]);
          break;
        }
        case DIAMOND:
          b.cornerPoly(
            [
              pt(px + 0.5, py),
              pt(px + 1, py + 0.5),
              pt(px + 0.5, py + 1),
              pt(px, py + 0.5),
            ],
            all4(sq),
          );
          break;
        case STAR:
          b.cornerPoly(
            [
              pt(px + 0.5, py),
              pt(px + 0.7, py + 0.3),
              pt(px + 1, py + 0.5),
              pt(px + 0.7, py + 0.7),
              pt(px + 0.5, py + 1),
              pt(px + 0.3, py + 0.7),
              pt(px, py + 0.5),
              pt(px + 0.3, py + 0.3),
            ],
            [sq, sq, sq, sq, sq, sq, sq, sq],
          );
          break;
      }
    }
  }
  return b.out;
}

function concave(
  b: Builder,
  g: Grid,
  x: number,
  y: number,
  px: number,
  py: number,
  c: Corner,
) {
  const r = c.r;
  const fills: [number[], number[], number[], Pt, Pt, Pt][] = [
    [[-1, 0], [0, -1], [-1, -1], pt(px, py), pt(r, 0), pt(0, r)],
    [[0, -1], [1, 0], [1, -1], pt(px + 1, py), pt(0, r), pt(-r, 0)],
    [[1, 0], [0, 1], [1, 1], pt(px + 1, py + 1), pt(-r, 0), pt(0, -r)],
    [[0, 1], [-1, 0], [-1, 1], pt(px, py + 1), pt(0, -r), pt(r, 0)],
  ];
  for (const [dx, dy, dd, p, toA, toB] of fills) {
    if (
      !g.on(x + dx[0], y + dx[1]) ||
      !g.on(x + dy[0], y + dy[1]) ||
      !g.on(x + dd[0], y + dd[1])
    )
      continue;
    const a = add(p, toA);
    const e = add(p, toB);
    b.move(p);
    b.line(a);
    if (c.kind === ROUND)
      b.cubic(
        add(a, scale(sub(p, a), KAPPA)),
        add(e, scale(sub(p, e), KAPPA)),
        e,
      );
    else b.line(e);
    b.close();
  }
}

function eyeFrame(shape: string): Contour[] {
  const b = new Builder();
  const ring = (o: Corner[], i: Corner[]) => {
    b.rect(0, 0, 7, 7, o);
    b.hole(1, 1, 5, 5, i);
  };
  const perimeter = (f: (x: number, y: number) => void) => {
    for (let i = 0; i < 7; i++) f(i, 0);
    for (let j = 1; j < 6; j++) f(6, j);
    for (let i = 6; i >= 0; i--) f(i, 6);
    for (let j = 5; j >= 1; j--) f(0, j);
  };
  switch (shape) {
    case "square":
      ring(all4(sq), all4(sq));
      break;
    case "rounded":
      ring(all4(rnd(1)), all4(rnd(1)));
      break;
    case "circle":
      ring(all4(rnd(3.5)), all4(rnd(2.5)));
      break;
    case "rounded-large":
      ring(all4(rnd(2.5)), all4(rnd(1.5)));
      break;
    case "rounded-outer":
      ring(all4(rnd(2)), all4(rnd(0.5)));
      break;
    case "rounded-inner":
      ring(all4(sq), all4(rnd(1.5)));
      break;
    case "leaf":
      ring([rnd(2.5), sq, rnd(2.5), sq], [rnd(1.5), sq, rnd(1.5), sq]);
      break;
    case "rounded-corner":
      ring([rnd(2.5), sq, sq, sq], [rnd(1.5), sq, sq, sq]);
      break;
    case "rounded-three":
      ring(
        [rnd(2.5), rnd(2.5), sq, rnd(2.5)],
        [rnd(1.5), rnd(1.5), sq, rnd(1.5)],
      );
      break;
    case "chamfer":
      ring(all4(cut(1.5)), all4(cut(0.914)));
      break;
    case "chamfer-leaf":
      ring([cut(2), sq, cut(2), sq], [cut(1.1), sq, cut(1.1), sq]);
      break;
    case "dotted":
      perimeter((x, y) => b.circle(x + 0.5, y + 0.5, 0.55));
      break;
    case "beaded":
      perimeter((x, y) =>
        b.rect(x + 0.05, y + 0.05, 0.9, 0.9, all4(rnd(0.25))),
      );
      break;
    case "concave": {
      const t = 1 + 5 / 3;
      const u = 1 + 10 / 3;
      b.rect(0, 0, 7, 7, all4(sq));
      b.move(pt(1, 1));
      b.cubic(pt(1.3, t), pt(1.3, u), pt(1, 6));
      b.cubic(pt(t, 5.7), pt(u, 5.7), pt(6, 6));
      b.cubic(pt(5.7, u), pt(5.7, t), pt(6, 1));
      b.cubic(pt(u, 1.3), pt(t, 1.3), pt(1, 1));
      b.close();
      break;
    }
    case "rough":
      b.rect(0.2, 0.2, 6.6, 6.6, all4(sq));
      b.hole(1, 1, 5, 5, all4(sq));
      for (let i = 1; i < 7; i += 2) {
        b.rect(i, 0, 1, 0.2, all4(sq));
        b.rect(6.8, i, 0.2, 1, all4(sq));
        b.rect(i, 6.8, 1, 0.2, all4(sq));
        b.rect(0, i, 0.2, 1, all4(sq));
      }
      break;
  }
  return b.out;
}

function eyeBall(shape: string): Contour[] {
  const b = new Builder();
  const box = (cs: Corner[]) => b.rect(2, 2, 3, 3, cs);
  switch (shape) {
    case "square":
      box(all4(sq));
      break;
    case "rounded":
      box(all4(rnd(1)));
      break;
    case "circle":
      box(all4(rnd(1.5)));
      break;
    case "rounded-soft":
      box(all4(rnd(0.5)));
      break;
    case "leaf":
      box([rnd(1.5), sq, rnd(1.5), sq]);
      break;
    case "rounded-corner":
      box([rnd(1.5), sq, sq, sq]);
      break;
    case "rounded-three":
      box([rnd(1.5), rnd(1.5), sq, rnd(1.5)]);
      break;
    case "chamfer":
      box(all4(cut(0.75)));
      break;
    case "chamfer-leaf":
      box([cut(0.9), sq, cut(0.9), sq]);
      break;
    case "dots-3x3":
      for (let j = 0; j < 3; j++)
        for (let i = 0; i < 3; i++)
          b.circle(2.6 + i * 0.9, 2.6 + j * 0.9, 0.66);
      break;
    case "capsules-horizontal":
      for (let j = 0; j < 3; j++) b.rect(2, 2 + j, 3, 1, all4(rnd(0.5)));
      break;
    case "capsules-vertical":
      for (let i = 0; i < 3; i++) b.rect(2 + i, 2, 1, 3, all4(rnd(0.5)));
      break;
    case "clover":
      b.circle(2.9, 2.9, 0.9);
      b.circle(4.1, 2.9, 0.9);
      b.circle(4.1, 4.1, 0.9);
      b.circle(2.9, 4.1, 0.9);
      break;
    case "star":
      b.cornerPoly(
        [
          pt(3.5, 2),
          pt(4.55, 2.45),
          pt(5, 3.5),
          pt(4.55, 4.55),
          pt(3.5, 5),
          pt(2.45, 4.55),
          pt(2, 3.5),
          pt(2.45, 2.45),
        ],
        [sq, sq, sq, sq, sq, sq, sq, sq],
      );
      break;
    case "concave":
      b.move(pt(2, 2));
      b.cubic(pt(3, 2.4), pt(4, 2.4), pt(5, 2));
      b.cubic(pt(4.6, 3), pt(4.6, 4), pt(5, 5));
      b.cubic(pt(4, 4.6), pt(3, 4.6), pt(2, 5));
      b.cubic(pt(2.4, 4), pt(2.4, 3), pt(2, 2));
      b.close();
      break;
    case "rough":
      b.rect(2.2, 2.2, 2.6, 2.6, all4(sq));
      for (const i of [2, 4]) {
        b.rect(i, 2, 1, 0.2, all4(sq));
        b.rect(4.8, i, 0.2, 1, all4(sq));
        b.rect(i, 4.8, 1, 0.2, all4(sq));
        b.rect(2, i, 0.2, 1, all4(sq));
      }
      break;
    case "cushion":
      b.move(pt(2, 2));
      b.cubic(pt(3, 1.8), pt(4, 1.8), pt(5, 2));
      b.cubic(pt(5.2, 3), pt(5.2, 4), pt(5, 5));
      b.cubic(pt(4, 5.2), pt(3, 5.2), pt(2, 5));
      b.cubic(pt(1.8, 4), pt(1.8, 3), pt(2, 2));
      b.close();
      break;
    case "cross": {
      const r = rnd(0.3);
      b.cornerPoly(
        [
          pt(2.45, 2),
          pt(4.55, 2),
          pt(4.55, 2.45),
          pt(5, 2.45),
          pt(5, 4.55),
          pt(4.55, 4.55),
          pt(4.55, 5),
          pt(2.45, 5),
          pt(2.45, 4.55),
          pt(2, 4.55),
          pt(2, 2.45),
          pt(2.45, 2.45),
        ],
        [r, r, sq, r, r, sq, r, r, sq, r, r, sq],
      );
      break;
    }
  }
  return b.out;
}

function placeEye(cs: Contour[], ox: number, oy: number, e: QrEye): Contour[] {
  return transform(
    cs,
    (p) => pt((e.flip_h ? 7 - p.x : p.x) + ox, (e.flip_v ? 7 - p.y : p.y) + oy),
    e.flip_h !== e.flip_v,
  );
}

// ---------------------------------------------------------------------------------------------
// Design and SVG (render.go)

const GRADIENT_ID = "qr-fill";

type Paint = { color: string; gradient: boolean };
type Layer = { contours: Contour[]; fill: Paint };
type Box = { x: number; y: number; w: number };

type Design = {
  n: number;
  q: number;
  total: number;
  background: string;
  style: QrStyleFull;
  layers: Layer[];
  tile: Box | null;
  logo: Box | null;
};

/** Accepts the API's matrix rows ("0101…") or a boolean grid. */
export type QrMatrix = readonly string[] | readonly (readonly boolean[])[];

function toGrid(matrix: QrMatrix): boolean[][] {
  return matrix.map((row) =>
    typeof row === "string"
      ? Array.from(row, (c) => c === "1")
      : Array.from(row),
  );
}

function buildDesign(m: boolean[][], s: QrStyleFull): Design {
  const n = m.length;
  const q = s.quiet_zone;
  const d: Design = {
    n,
    q,
    total: n + 2 * q,
    background: s.transparent_background ? "" : s.background,
    style: s,
    layers: [],
    tile: null,
    logo: null,
  };
  let cleared: (x: number, y: number) => boolean = () => false;
  if (s.logo_profile_uuid) {
    const w = (n * s.logo_size) / 100;
    d.logo = { x: (n - w) / 2 + q, y: (n - w) / 2 + q, w };
    if (s.logo_mode === "clean") {
      const tw = w + 2;
      const t0 = (n - tw) / 2;
      const t1 = t0 + tw;
      if (s.transparent_background) {
        cleared = (x, y) =>
          x + 0.5 > t0 && x + 0.5 < t1 && y + 0.5 > t0 && y + 0.5 < t1;
      } else {
        cleared = (x, y) => x >= t0 && x + 1 <= t1 && y >= t0 && y + 1 <= t1;
        d.tile = { x: t0 + q, y: t0 + q, w: tw };
      }
    }
  }
  const off = (x: number, y: number) => inFinder(n, x, y) || cleared(x, y);
  const g: Grid = {
    n,
    off,
    on: (x, y) => x >= 0 && y >= 0 && x < n && y < n && m[y][x] && !off(x, y),
  };
  const gradient = s.fill_mode !== "solid";
  d.layers.push({
    contours: bodyGeometry(g, s.body_shape, q),
    fill: { color: s.foreground, gradient },
  });
  const eyePaint = (c: string): Paint =>
    c
      ? { color: c, gradient: false }
      : { color: s.foreground, gradient: gradient && s.gradient_on_eyes };
  const frame = eyeFrame(s.eye_frame_shape);
  const ball = eyeBall(s.eye_ball_shape);
  const positions = [
    [0, 0],
    [n - 7, 0],
    [0, n - 7],
  ];
  positions.forEach(([px, py], i) => {
    const e = s.eyes[i];
    const ox = px + q;
    const oy = py + q;
    d.layers.push(
      { contours: placeEye(frame, ox, oy, e), fill: eyePaint(e.frame_color) },
      { contours: placeEye(ball, ox, oy, e), fill: eyePaint(e.ball_color) },
    );
  });
  return d;
}

function escapeAttr(v: string): string {
  return v
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function svgDocument(d: Design, size: number, logoHref: string): string {
  const s = d.style;
  const t = fmtNum(d.total);
  let out = `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 ${t} ${t}">`;
  if (s.fill_mode !== "solid") {
    const stops = `<stop offset="0" stop-color="${s.gradient_start}"/><stop offset="1" stop-color="${s.gradient_end}"/>`;
    const c = fmtNum(d.q + d.n / 2);
    if (s.fill_mode === "linear") {
      out += `<defs><linearGradient id="${GRADIENT_ID}" gradientUnits="userSpaceOnUse" x1="${fmtNum(d.q)}" y1="${c}" x2="${fmtNum(d.q + d.n)}" y2="${c}" gradientTransform="rotate(${fmtNum(s.gradient_angle)} ${c} ${c})">${stops}</linearGradient></defs>`;
    } else {
      out += `<defs><radialGradient id="${GRADIENT_ID}" gradientUnits="userSpaceOnUse" cx="${c}" cy="${c}" r="${fmtNum(d.n * 0.7071067811865476)}">${stops}</radialGradient></defs>`;
    }
  }
  if (d.background)
    out += `<rect width="${t}" height="${t}" fill="${d.background}"/>`;
  for (const l of d.layers) {
    if (l.contours.length === 0) continue;
    const fill = l.fill.gradient ? `url(#${GRADIENT_ID})` : l.fill.color;
    out += `<path fill="${fill}" d="${pathData(l.contours)}"/>`;
  }
  if (d.tile)
    out += `<rect x="${fmtNum(d.tile.x)}" y="${fmtNum(d.tile.y)}" width="${fmtNum(d.tile.w)}" height="${fmtNum(d.tile.w)}" fill="${s.background}"/>`;
  if (d.logo && logoHref)
    out += `<image x="${fmtNum(d.logo.x)}" y="${fmtNum(d.logo.y)}" width="${fmtNum(d.logo.w)}" height="${fmtNum(d.logo.w)}" preserveAspectRatio="xMidYMid meet" href="${escapeAttr(logoHref)}"/>`;
  return out + "</svg>";
}

/**
 * The SVG the Go renderer produces for the same matrix and style (qr.RenderSVG). logoHref is
 * the image used when the style has a logo; pass a data: URL so the file stands alone.
 */
export function renderQrSvg(
  matrix: QrMatrix,
  style: QrStyle,
  size: number,
  logoHref = "",
): string {
  return svgDocument(
    buildDesign(toGrid(matrix), normalizeQrStyle(style)),
    size,
    logoHref,
  );
}

/** Logo box in module units (for overlays), or null without a logo. */
export function qrLogoBox(
  matrixSize: number,
  style: QrStyle,
): { x: number; y: number; w: number; total: number } | null {
  const s = normalizeQrStyle(style);
  if (!s.logo_profile_uuid) return null;
  const w = (matrixSize * s.logo_size) / 100;
  const q = s.quiet_zone;
  return {
    x: (matrixSize - w) / 2 + q,
    y: (matrixSize - w) / 2 + q,
    w,
    total: matrixSize + 2 * q,
  };
}

/**
 * Path data of a single shape, for editor swatches: a body sample on a small fixed pattern or
 * one eye part, all in a 0..7 viewBox.
 */
export function shapeIconPath(
  kind: "body" | "frame" | "ball",
  shape: string,
): string {
  if (kind === "frame") return pathData(eyeFrame(shape));
  if (kind === "ball") return pathData(eyeBall(shape));
  const sample = [
    "1101100",
    "1111011",
    "0100110",
    "1101111",
    "1011010",
    "1110110",
    "0101101",
  ].map((r) => Array.from(r, (c) => c === "1"));
  const g: Grid = {
    n: 7,
    off: () => false,
    on: (x, y) => x >= 0 && y >= 0 && x < 7 && y < 7 && sample[y][x],
  };
  return pathData(bodyGeometry(g, shape, 0));
}
