import type { CSSProperties } from "react";

/**
 * Company brand color on employee public pages (account_branding, TEC-946). Applied after the
 * theme tokens, it replaces only the primary color; the text on it is white or near-black,
 * whichever contrasts more.
 */
const HEX_RE = /^#([0-9a-f]{6})$/i;

function luminance(hex: string): number {
  const n = parseInt(hex.slice(1), 16);
  const channel = (v: number) => {
    const c = v / 255;
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  };
  return (
    0.2126 * channel((n >> 16) & 255) +
    0.7152 * channel((n >> 8) & 255) +
    0.0722 * channel(n & 255)
  );
}

const contrast = (a: number, b: number) =>
  (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

/** CSS variables for a #rrggbb brand color; empty for "" or a malformed value. */
export function brandStyle(color: string | null | undefined): CSSProperties {
  if (!color || !HEX_RE.test(color)) return {};
  const l = luminance(color);
  const foreground =
    contrast(l, 1) >= contrast(l, luminance("#111827")) ? "#ffffff" : "#111827";
  return {
    "--primary": color,
    "--primary-foreground": foreground,
    "--ring": color,
    "--comcode-gradient": `linear-gradient(135deg, ${color} 0%, ${color} 100%)`,
  } as CSSProperties;
}
