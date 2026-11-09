import type {
  AspectRatioType,
  BackgroundDocument,
  BackgroundKind,
  GradientBackground,
  ShaderBackground,
  SolidBackground,
  VideoBackground,
} from "../types";
import { SHADER_REGISTRY } from "../data/shaders-registry";
import type { ShaderType } from "../data/shader-types";

export const COLOR_PALETTES: string[][] = [
  ["#e0eaff", "#241d9a", "#f75092", "#9f50d3"],
  ["#bcecf6", "#00aaff", "#00f7ff", "#ffd447"],
  ["#ff4e50", "#f9d423", "#e1eec3", "#f05053"],
  ["#111827", "#4f46e5", "#7c3aed", "#ec4899"],
  ["#000000", "#1e293b", "#3b82f6", "#06b6d4"],
  ["#0f172a", "#38bdf8", "#818cf8", "#c084fc"],
  ["#18181b", "#22c55e", "#10b981", "#14b8a6"],
  ["#2e1065", "#d946ef", "#ec4899", "#f43f5e"],
];

export function getRandomColorPalette(): string[] {
  const index = Math.floor(Math.random() * COLOR_PALETTES.length);
  return [...COLOR_PALETTES[index]];
}

export function generateRandomHex(): string {
  const chars = "0123456789ABCDEF";
  let color = "#";
  for (let i = 0; i < 6; i++) {
    color += chars[Math.floor(Math.random() * chars.length)];
  }
  return color;
}

export function newBackgroundId(kind: BackgroundKind): string {
  return `bg-${kind}-${Date.now()}`;
}

export function createSolidBackground(
  overrides?: Partial<SolidBackground>,
): SolidBackground {
  const { params: paramOverrides, ...rest } = overrides ?? {};
  return {
    version: "1.0",
    id: newBackgroundId("solid"),
    name: "Solid Background",
    kind: "solid",
    aspectRatio: "responsive",
    createdAt: new Date().toISOString(),
    ...rest,
    params: { color: "#0f172a", ...paramOverrides },
  };
}

export function createGradientBackground(
  overrides?: Partial<GradientBackground>,
): GradientBackground {
  const palette = COLOR_PALETTES[0];
  const { params: paramOverrides, ...rest } = overrides ?? {};
  return {
    version: "1.0",
    id: newBackgroundId("gradient"),
    name: "Gradient Background",
    kind: "gradient",
    aspectRatio: "responsive",
    createdAt: new Date().toISOString(),
    ...rest,
    params: {
      type: "linear",
      colors: [...palette],
      angle: 135,
      ...paramOverrides,
    },
  };
}

export function createVideoBackground(
  overrides?: Partial<VideoBackground>,
): VideoBackground {
  const { params: paramOverrides, ...rest } = overrides ?? {};
  return {
    version: "1.0",
    id: newBackgroundId("video"),
    name: "Video Background",
    kind: "video",
    aspectRatio: "responsive",
    createdAt: new Date().toISOString(),
    ...rest,
    params: {
      src: "",
      muted: true,
      loop: true,
      autoplay: true,
      objectFit: "cover",
      ...paramOverrides,
    },
  };
}

export function createShaderBackground(
  shaderType: ShaderType = "mesh-gradient",
  overrides?: Partial<ShaderBackground>,
): ShaderBackground {
  const entry = SHADER_REGISTRY[shaderType] ?? SHADER_REGISTRY["mesh-gradient"];
  const { params: paramOverrides, ...rest } = overrides ?? {};
  return {
    version: "1.0",
    id: newBackgroundId("shader"),
    name: `${entry.name} Background`,
    kind: "shader",
    aspectRatio: "responsive",
    createdAt: new Date().toISOString(),
    shaderType: entry.id as ShaderType,
    category: entry.category,
    ...rest,
    params: { ...entry.defaultParams, ...paramOverrides },
  };
}

export function createDefaultDocument(
  kind: BackgroundKind = "shader",
): BackgroundDocument {
  switch (kind) {
    case "solid":
      return createSolidBackground();
    case "gradient":
      return createGradientBackground();
    case "video":
      return createVideoBackground();
    case "shader":
    default:
      return createShaderBackground();
  }
}

export function aspectRatioClass(
  aspectRatio: AspectRatioType | undefined,
  fullscreen: boolean,
): string {
  if (fullscreen) return "h-full w-full";
  switch (aspectRatio) {
    case "16:9":
      return "aspect-video w-full max-h-[70vh]";
    case "9:16":
      return "aspect-[9/16] h-[70vh] max-w-full";
    case "1:1":
      return "aspect-square h-[70vh] max-w-full";
    case "4:3":
      return "aspect-[4/3] w-full max-h-[70vh]";
    default:
      return "min-h-[420px] w-full h-full";
  }
}
