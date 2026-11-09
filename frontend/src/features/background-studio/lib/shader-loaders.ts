import type { ComponentType } from "react";

import type { ShaderType } from "../data/shader-types";

// Heterogeneous loaders: each shader component has its own props type.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type ShaderComponent = ComponentType<any>;

/**
 * One dynamic import per shader so a public page only downloads the shader it renders (TEC-600).
 * Each target is a one-line re-export module: `import(pkg).then((m) => m.X)` would pull the whole
 * barrel (all 30 shaders) into one chunk, while a static named re-export is tree-shaken
 * (the package is `sideEffects: false`).
 * Keep this module free of `shaders-registry` (presets + all 30 components): that is studio-only.
 */
export const SHADER_LOADERS: Record<ShaderType, () => Promise<ShaderComponent>> = {
  "mesh-gradient": () => import("../data/shader-chunks/mesh-gradient").then((m) => m.default),
  "static-mesh-gradient": () => import("../data/shader-chunks/static-mesh-gradient").then((m) => m.default),
  "static-radial-gradient": () => import("../data/shader-chunks/static-radial-gradient").then((m) => m.default),
  "grain-gradient": () => import("../data/shader-chunks/grain-gradient").then((m) => m.default),
  warp: () => import("../data/shader-chunks/warp").then((m) => m.default),
  swirl: () => import("../data/shader-chunks/swirl").then((m) => m.default),
  spiral: () => import("../data/shader-chunks/spiral").then((m) => m.default),
  "god-rays": () => import("../data/shader-chunks/god-rays").then((m) => m.default),
  "pulsing-border": () => import("../data/shader-chunks/pulsing-border").then((m) => m.default),
  "color-panels": () => import("../data/shader-chunks/color-panels").then((m) => m.default),
  "smoke-ring": () => import("../data/shader-chunks/smoke-ring").then((m) => m.default),
  dithering: () => import("../data/shader-chunks/dithering").then((m) => m.default),
  "dot-grid": () => import("../data/shader-chunks/dot-grid").then((m) => m.default),
  "dot-orbit": () => import("../data/shader-chunks/dot-orbit").then((m) => m.default),
  metaballs: () => import("../data/shader-chunks/metaballs").then((m) => m.default),
  "neuro-noise": () => import("../data/shader-chunks/neuro-noise").then((m) => m.default),
  "simplex-noise": () => import("../data/shader-chunks/simplex-noise").then((m) => m.default),
  "perlin-noise": () => import("../data/shader-chunks/perlin-noise").then((m) => m.default),
  voronoi: () => import("../data/shader-chunks/voronoi").then((m) => m.default),
  waves: () => import("../data/shader-chunks/waves").then((m) => m.default),
  "paper-texture": () => import("../data/shader-chunks/paper-texture").then((m) => m.default),
  "fluted-glass": () => import("../data/shader-chunks/fluted-glass").then((m) => m.default),
  water: () => import("../data/shader-chunks/water").then((m) => m.default),
  "image-dithering": () => import("../data/shader-chunks/image-dithering").then((m) => m.default),
  "halftone-dots": () => import("../data/shader-chunks/halftone-dots").then((m) => m.default),
  "halftone-cmyk": () => import("../data/shader-chunks/halftone-cmyk").then((m) => m.default),
  "lens-distortion": () => import("../data/shader-chunks/lens-distortion").then((m) => m.default),
  "liquid-metal": () => import("../data/shader-chunks/liquid-metal").then((m) => m.default),
  heatmap: () => import("../data/shader-chunks/heatmap").then((m) => m.default),
  "gem-smoke": () => import("../data/shader-chunks/gem-smoke").then((m) => m.default),
};

export function isKnownShaderType(value: unknown): value is ShaderType {
  return typeof value === "string" && Object.prototype.hasOwnProperty.call(SHADER_LOADERS, value);
}
