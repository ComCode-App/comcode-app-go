/**
 * Route QR editor helpers (TEC-1270) on top of the shared design engine (qr-engine.ts).
 * The API stays the authority: qrStyleProblems mirrors qr.Style.Validate for instant feedback.
 *
 * Type-only imports: node --test loads this file directly with type stripping.
 */
import type { QrEye, QrStyle, QrStyleFull } from "./qr-engine";

export const DOWNLOAD_FORMATS = ["svg", "png", "pdf"] as const;
export const DOWNLOAD_SIZES = [512, 1024, 2048] as const;
export type DownloadFormat = (typeof DOWNLOAD_FORMATS)[number];

/**
 * The default design (TEC-1273), equal to qr.DefaultStyle in Go: what a never-styled route is
 * drawn with and what "reset to default" selects. v1 fields stay empty; the normalizer derives them.
 */
export const DEFAULT_QR_STYLE: QrStyle = {
  foreground: "#000000",
  background: "#ffffff",
  module: "",
  eye: "",
  body_shape: "rounded-pointed",
  eye_frame_shape: "rounded-large",
  eye_ball_shape: "rounded",
  quiet_zone: 4,
  logo_profile_uuid: null,
  logo_field: "",
};

export type QrTemplate = { id: string; style: Partial<QrStyle> };

const DESIGN_DEFAULTS: Partial<QrStyle> = {
  transparent_background: false,
  body_shape: "square",
  eye_frame_shape: "square",
  eye_ball_shape: "square",
  fill_mode: "solid",
  gradient_angle: 0,
  gradient_on_eyes: false,
};

const blankEyes = (): QrEye[] =>
  [0, 1, 2].map(() => ({
    flip_h: false,
    flip_v: false,
    frame_color: "",
    ball_color: "",
  }));

/**
 * A template replaces the look (shapes, colors, flips, background) and keeps what belongs to the
 * route: the logo choice, its size and mode, and the quiet zone. v1 fields are cleared so the
 * normalizer derives them from the template's v2 shapes.
 */
export function applyTemplate(current: QrStyle, template: QrTemplate): QrStyle {
  const style = template.style;
  return {
    ...DESIGN_DEFAULTS,
    foreground: "#000000",
    background: "#ffffff",
    ...style,
    eyes: style.eyes ?? blankEyes(),
    gradient_start: style.gradient_start ?? style.foreground ?? "#000000",
    gradient_end: style.gradient_end ?? style.foreground ?? "#000000",
    module: "",
    eye: "",
    quiet_zone: current.quiet_zone,
    logo_profile_uuid: current.logo_profile_uuid ?? null,
    logo_field: current.logo_field ?? "",
    logo_size: current.logo_size,
    logo_mode: current.logo_mode,
  };
}

/** Whether the normalized style looks like the template (for the picker's selected state). */
export function matchesTemplate(
  style: QrStyleFull,
  template: QrStyleFull,
): boolean {
  const keys: (keyof QrStyleFull)[] = [
    "foreground",
    "background",
    "transparent_background",
    "body_shape",
    "eye_frame_shape",
    "eye_ball_shape",
    "fill_mode",
    "gradient_on_eyes",
    "eyes",
  ];
  if (
    keys.some((k) => JSON.stringify(style[k]) !== JSON.stringify(template[k]))
  )
    return false;
  if (style.fill_mode === "solid") return true;
  return (
    style.gradient_start === template.gradient_start &&
    style.gradient_end === template.gradient_end &&
    (style.fill_mode === "radial" ||
      style.gradient_angle === template.gradient_angle)
  );
}

/** Mirrors the top-right eye horizontally and the bottom-left one vertically. */
export function mirroredEyes(eyes: QrEye[]): QrEye[] {
  return eyes.map((e, i) => ({
    ...e,
    flip_h: i === 1,
    flip_v: i === 2,
  }));
}

export type EyeColorMode = "inherit" | "shared" | "per_eye";

export function eyeColorMode(eyes: QrEye[]): EyeColorMode {
  if (eyes.every((e) => !e.frame_color && !e.ball_color)) return "inherit";
  const [a, ...rest] = eyes;
  if (
    rest.every(
      (e) => e.frame_color === a.frame_color && e.ball_color === a.ball_color,
    )
  )
    return "shared";
  return "per_eye";
}

export type QrWarning =
  "low_margin" | "large_logo" | "transparent" | "logo_overlay" | "dense_shape";

const BORDERLINE_CONTRAST = 7;

/**
 * Advice beyond the hard rules: the API accepts these styles, and the readability tests decode
 * every shape, but each item narrows the margin real cameras have.
 */
export function readabilityWarnings(
  s: QrStyleFull,
  contrasts: number[],
): QrWarning[] {
  const out: QrWarning[] = [];
  if (contrasts.some((c) => c >= 4.5 && c < BORDERLINE_CONTRAST))
    out.push("low_margin");
  if (s.logo_profile_uuid && s.logo_size > 22) out.push("large_logo");
  if (s.logo_profile_uuid && s.logo_mode === "overlay")
    out.push("logo_overlay");
  if (s.transparent_background) out.push("transparent");
  if (["small-dot", "spaced-square", "star", "diamond"].includes(s.body_shape))
    out.push("dense_shape");
  return out;
}

/** Matrix rows for the current correction level: H with a logo, M without. */
export function matrixFor(
  matrices: { M: string[]; H: string[] } | undefined,
  hasLogo: boolean,
): string[] | null {
  if (!matrices) return null;
  const rows = hasLogo ? matrices.H : matrices.M;
  return rows && rows.length >= 21 ? rows : null;
}
