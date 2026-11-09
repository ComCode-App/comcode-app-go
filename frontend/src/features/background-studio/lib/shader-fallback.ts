import type { CSSProperties } from "react";

/**
 * Static stand-in for a shader background (TEC-600): painted immediately (SSR, before any JS) and
 * kept when the device cannot or should not run WebGL. Colors come from the shader's own params
 * (`colorBack`, `colors`, `colorFront`, ...) so the fallback matches the design's palette.
 */

const COLOR_RE = /^(#[0-9a-f]{3,8}|(rgb|rgba|hsl|hsla|oklch|oklab|lab|lch)\([^;{}]*\))$/i;

function isColor(value: unknown): value is string {
  return typeof value === "string" && COLOR_RE.test(value.trim());
}

/** Palette of a shader config, in a stable order: background first, then the main colors. */
export function shaderPalette(params: Record<string, unknown>): { back?: string; colors: string[] } {
  const back = isColor(params.colorBack) ? params.colorBack : undefined;
  const colors: string[] = [];
  const push = (value: unknown) => {
    if (isColor(value) && !colors.includes(value)) colors.push(value);
  };
  if (Array.isArray(params.colors)) params.colors.forEach(push);
  for (const key of ["colorFront", "colorInner", "colorTint", "borderColor"]) push(params[key]);
  for (const [key, value] of Object.entries(params)) {
    if (/^color/i.test(key) && key !== "colorBack") push(value);
  }
  return { back, colors };
}

export function shaderFallbackStyle(params: Record<string, unknown>): CSSProperties {
  const { back, colors } = shaderPalette(params);
  const base = back ?? colors[0] ?? "#0f172a";
  if (colors.length === 0) return { backgroundColor: base };
  if (colors.length === 1) {
    return {
      backgroundColor: base,
      backgroundImage: `radial-gradient(circle at 50% 40%, ${colors[0]}, transparent 70%)`,
    };
  }
  // Soft mesh look: one radial blob per color over the base, mirroring mesh/grain gradients.
  const spots = [
    "20% 20%",
    "80% 25%",
    "75% 80%",
    "20% 75%",
    "50% 50%",
  ];
  const layers = colors
    .slice(0, spots.length)
    .map((color, i) => `radial-gradient(circle at ${spots[i]}, ${color}, transparent 60%)`);
  // Without an explicit back color, a diagonal blend of the palette is the bottom layer.
  if (!back) layers.push(`linear-gradient(135deg, ${colors.join(", ")})`);
  return { backgroundColor: base, backgroundImage: layers.join(", ") };
}
