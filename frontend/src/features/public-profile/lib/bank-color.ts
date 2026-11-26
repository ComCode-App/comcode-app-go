/**
 * Bank card colors on the public page (TEC-1314). The card takes the bank's color (taken from
 * its logo, else the bank's stored brand color) and picks text and surface colors from it so
 * every text keeps WCAG AA (4.5:1) on both gradient stops.
 *
 * Pure functions only: `bank-color.test.mjs` imports this file directly.
 */

export type RGB = { r: number; g: number; b: number };

export type BankCardColors = {
  /** Card background (CSS gradient). */
  background: string;
  /** Text and icon color on the card. */
  foreground: string;
  /** Secondary text (account holder label). */
  muted: string;
  /** Translucent chip behind the IBAN and the copy buttons. */
  surface: string;
};

const MIN_CONTRAST = 4.5;
const WHITE: RGB = { r: 255, g: 255, b: 255 };
const INK: RGB = { r: 15, g: 17, b: 23 };

export function parseHex(hex: string | null | undefined): RGB | null {
  const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec((hex ?? "").trim());
  if (!m) return null;
  let h = m[1];
  if (h.length === 3) h = [...h].map((c) => c + c).join("");
  const n = parseInt(h, 16);
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 };
}

export function toHex({ r, g, b }: RGB): string {
  const c = (v: number) =>
    Math.round(Math.max(0, Math.min(255, v)))
      .toString(16)
      .padStart(2, "0");
  return `#${c(r)}${c(g)}${c(b)}`;
}

function luminance({ r, g, b }: RGB): number {
  const ch = (v: number) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * ch(r) + 0.7152 * ch(g) + 0.0722 * ch(b);
}

export function contrast(a: RGB, b: RGB): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

/** Mixes `color` toward `target` by `amount` (0..1). */
function mix(color: RGB, target: RGB, amount: number): RGB {
  return {
    r: color.r + (target.r - color.r) * amount,
    g: color.g + (target.g - color.g) * amount,
    b: color.b + (target.b - color.b) * amount,
  };
}

function rgba({ r, g, b }: RGB, alpha: number): string {
  return `rgba(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)}, ${alpha})`;
}

/**
 * Dominant color of a logo from its RGBA pixels: transparent, near-white, near-black and grey
 * pixels are skipped (logo backgrounds and outlines), the rest is grouped by coarse hue and the
 * fullest group is averaged. Null when the logo has no such color (a black or white logo).
 */
export function dominantColor(pixels: ArrayLike<number>): RGB | null {
  const buckets = new Map<
    number,
    { r: number; g: number; b: number; n: number }
  >();
  for (let i = 0; i + 3 < pixels.length; i += 4) {
    const r = pixels[i];
    const g = pixels[i + 1];
    const b = pixels[i + 2];
    const a = pixels[i + 3];
    if (a < 128) continue;
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    if (max > 245 && min > 230) continue; // white
    if (max < 25) continue; // black
    if (max - min < 28) continue; // grey
    const key = ((r >> 5) << 6) | ((g >> 5) << 3) | (b >> 5);
    const bucket = buckets.get(key) ?? { r: 0, g: 0, b: 0, n: 0 };
    bucket.r += r;
    bucket.g += g;
    bucket.b += b;
    bucket.n += 1;
    buckets.set(key, bucket);
  }
  let best: { r: number; g: number; b: number; n: number } | null = null;
  for (const bucket of buckets.values()) {
    if (!best || bucket.n > best.n) best = bucket;
  }
  if (!best || best.n < 4) return null;
  return { r: best.r / best.n, g: best.g / best.n, b: best.b / best.n };
}

/**
 * Card colors for a bank color: a gradient from the color to a darker shade, and white or ink
 * text, whichever reads better. When neither passes 4.5:1 on both gradient stops, the base
 * is darkened or lightened in small steps until one does, so mid-tone brand colors (green)
 * and light ones (yellow) both work.
 */
export function bankCardColors(color: RGB): BankCardColors {
  // Smallest change first; at equal steps a darker card (white text) wins over a lighter one.
  for (let step = 0; step <= 12; step++) {
    const candidates =
      step === 0
        ? [color]
        : [mix(color, INK, step * 0.07), mix(color, WHITE, step * 0.07)];
    for (const base of candidates) {
      const colors = cardColorsFor(base);
      if (colors) return colors;
    }
  }
  return {
    background: "linear-gradient(135deg, #1f2937 0%, #111827 100%)",
    foreground: "#ffffff",
    muted: rgba(WHITE, MUTED_MIX),
    surface: rgba(WHITE, 0.16),
  };
}

/** Card colors when `base` passes 4.5:1 for every text with white or ink; else null. */
function cardColorsFor(base: RGB): BankCardColors | null {
  const end = mix(base, INK, 0.28);
  for (const fg of preferredText(base, end)) {
    const light = fg === WHITE;
    const chipColor = light ? WHITE : INK;
    const chipAlpha = light ? 0.16 : 0.1;
    const ok = [base, end].every(
      (stop) =>
        contrast(fg, stop) >= MIN_CONTRAST &&
        contrast(mix(stop, fg, MUTED_MIX), stop) >= MIN_CONTRAST &&
        contrast(fg, mix(stop, chipColor, chipAlpha)) >= MIN_CONTRAST,
    );
    if (ok) {
      return {
        background: `linear-gradient(135deg, ${toHex(base)} 0%, ${toHex(end)} 100%)`,
        foreground: toHex(fg),
        muted: rgba(fg, MUTED_MIX),
        surface: rgba(chipColor, chipAlpha),
      };
    }
  }
  return null;
}

/** Opacity of secondary text over the card. */
const MUTED_MIX = 0.9;

/** White or ink, the one with more contrast on the gradient first. */
function preferredText(base: RGB, end: RGB): RGB[] {
  const white = Math.min(contrast(WHITE, base), contrast(WHITE, end));
  const ink = Math.min(contrast(INK, base), contrast(INK, end));
  return white >= ink ? [WHITE, INK] : [INK, WHITE];
}
