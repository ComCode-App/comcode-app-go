import QRCode from "qrcode";

import type { RouteQRStyle } from "../services/route-qr.service";

/** Mirrors backend/internal/platform/qr: the API stays the authority, this is instant feedback. */
export const MIN_CONTRAST = 4.5;
export const QUIET_ZONE_MIN = 4;
export const QUIET_ZONE_MAX = 16;
export const MODULE_STYLES = ["square", "rounded", "dots"] as const;
export const EYE_STYLES = ["square", "rounded", "circle"] as const;
export const DOWNLOAD_FORMATS = ["svg", "png", "pdf"] as const;
export const DOWNLOAD_SIZES = [512, 1024, 2048] as const;
export type DownloadFormat = (typeof DOWNLOAD_FORMATS)[number];

export const DEFAULT_QR_STYLE: RouteQRStyle = {
  foreground: "#000000",
  background: "#ffffff",
  module: "square",
  eye: "square",
  quiet_zone: 4,
  logo_profile_uuid: null,
  logo_field: "",
};

const HEX = /^#[0-9a-fA-F]{6}$/;

export function isHexColor(value: string): boolean {
  return HEX.test(value);
}

function luminance(hex: string): number {
  const v = Number.parseInt(hex.slice(1), 16);
  const linear = (c: number) => {
    const x = c / 255;
    return x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * linear((v >> 16) & 255) + 0.7152 * linear((v >> 8) & 255) + 0.0722 * linear(v & 255);
}

/**
 * Directional contrast (background over foreground), like the API: light-on-dark
 * ("inverted") codes score below 1 and are rejected because many scanners fail on them.
 */
export function qrContrast(foreground: string, background: string): number | null {
  if (!isHexColor(foreground) || !isHexColor(background)) return null;
  return (luminance(background) + 0.05) / (luminance(foreground) + 0.05);
}

export type StyleProblem = "color_format" | "contrast" | "inverted" | "logo_field";

export function styleProblem(style: RouteQRStyle): StyleProblem | null {
  const ratio = qrContrast(style.foreground, style.background);
  if (ratio === null) return "color_format";
  if (ratio < 1) return "inverted";
  if (ratio < MIN_CONTRAST) return "contrast";
  if (style.logo_profile_uuid && style.logo_field !== "avatar" && style.logo_field !== "cover") return "logo_field";
  return null;
}

/** Normalizes the server's stored style (null/empty logo fields) for comparison and PUT. */
export function normalizeStyle(style: RouteQRStyle): RouteQRStyle {
  const hasLogo = Boolean(style.logo_profile_uuid);
  return {
    foreground: style.foreground.toLowerCase(),
    background: style.background.toLowerCase(),
    module: style.module,
    eye: style.eye,
    quiet_zone: style.quiet_zone,
    logo_profile_uuid: hasLogo ? style.logo_profile_uuid : null,
    logo_field: hasLogo ? style.logo_field || "avatar" : "",
  };
}

export function sameStyle(a: RouteQRStyle, b: RouteQRStyle): boolean {
  return JSON.stringify(normalizeStyle(a)) === JSON.stringify(normalizeStyle(b));
}

export type QrShape = { x: number; y: number; w: number; r: number; fill: string };
export type QrGeometry = { total: number; shapes: QrShape[]; logo: { pos: number; w: number } | null };

/**
 * Local live preview with the same geometry as the server renderer: styled data modules,
 * three finder "eyes" drawn as 7/5/3 nested squares, and a background tile behind the logo.
 * The bit pattern may differ from the server's (mask choice), the look does not.
 */
export function previewGeometry(payload: string, style: RouteQRStyle): QrGeometry | null {
  if (styleProblem(style) === "color_format") return null;
  const hasLogo = Boolean(style.logo_profile_uuid);
  let matrix: { size: number; get: (row: number, col: number) => number };
  try {
    matrix = QRCode.create(payload, { errorCorrectionLevel: hasLogo ? "H" : "M" }).modules;
  } catch {
    return null;
  }
  const n = matrix.size;
  const q = style.quiet_zone;
  const total = n + 2 * q;
  const radius = style.module === "rounded" ? 0.2 : style.module === "dots" ? 0.5 : 0;
  const shapes: QrShape[] = [];
  for (let y = 0; y < n; y++) {
    for (let x = 0; x < n; x++) {
      const finder = (x < 7 && y < 7) || (x >= n - 7 && y < 7) || (x < 7 && y >= n - 7);
      if (!finder && matrix.get(y, x)) shapes.push({ x: x + q, y: y + q, w: 1, r: radius, fill: style.foreground });
    }
  }
  for (const [px, py] of [
    [0, 0],
    [n - 7, 0],
    [0, n - 7],
  ]) {
    [7, 5, 3].forEach((w, i) => {
      const r = style.eye === "rounded" ? Math.min(1, w / 2) : style.eye === "circle" ? w / 2 : 0;
      shapes.push({ x: px + i + q, y: py + i + q, w, r, fill: i === 1 ? style.background : style.foreground });
    });
  }
  if (!hasLogo) return { total, shapes, logo: null };
  const logoW = n * 0.15;
  const tile = logoW + 2;
  shapes.push({ x: (total - tile) / 2, y: (total - tile) / 2, w: tile, r: 0, fill: style.background });
  return { total, shapes, logo: { pos: (total - logoW) / 2, w: logoW } };
}
