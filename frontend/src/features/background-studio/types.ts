/**
 * Unified JSON background document. Saved via /v1/backgrounds (TEC-599); the API validates it.
 * Four kinds: solid | gradient | video | shader (Paper Design).
 */

import type {
  AspectRatioType,
  ShaderCategory,
  ShaderType,
} from "./data/shader-types";

export type { AspectRatioType, ShaderCategory, ShaderType };

export type BackgroundKind = "solid" | "gradient" | "video" | "shader";

export type GradientType = "linear" | "radial";

export type VideoObjectFit = "cover" | "contain" | "fill";

export interface BackgroundMeta {
  tags?: string[];
  author?: string;
  description?: string;
}

interface BackgroundBase {
  version: "1.0";
  id: string;
  name: string;
  kind: BackgroundKind;
  aspectRatio?: AspectRatioType;
  createdAt?: string;
  metadata?: BackgroundMeta;
}

export interface SolidParams {
  color: string;
}

export interface GradientParams {
  type: GradientType;
  colors: string[];
  angle?: number;
}

export interface VideoParams {
  src: string;
  poster?: string;
  muted?: boolean;
  loop?: boolean;
  autoplay?: boolean;
  objectFit?: VideoObjectFit;
}

export interface ShaderParamsBlock {
  shaderType: ShaderType;
  category: ShaderCategory;
  params: Record<string, unknown>;
}

export type SolidBackground = BackgroundBase & {
  kind: "solid";
  params: SolidParams;
};

export type GradientBackground = BackgroundBase & {
  kind: "gradient";
  params: GradientParams;
};

export type VideoBackground = BackgroundBase & {
  kind: "video";
  params: VideoParams;
};

export type ShaderBackground = BackgroundBase & {
  kind: "shader";
  shaderType: ShaderType;
  category: ShaderCategory;
  params: Record<string, unknown>;
};

export type BackgroundDocument =
  | SolidBackground
  | GradientBackground
  | VideoBackground
  | ShaderBackground;

/** Legacy furkan schema (shader-only, no `kind`). */
export interface LegacyShaderConfig {
  version?: string;
  id?: string;
  name?: string;
  shaderType: string;
  category?: string;
  aspectRatio?: AspectRatioType;
  params?: Record<string, unknown>;
  createdAt?: string;
  metadata?: BackgroundMeta;
}
