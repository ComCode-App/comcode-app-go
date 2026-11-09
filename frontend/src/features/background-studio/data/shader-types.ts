/**
 * ============================================================================
 * SHADER STUDIO & BACKGROUND GENERATOR - COMPLETE TYPES DEFINITIONS
 * ============================================================================
 * These types can be directly imported into any external React/TypeScript project.
 */

// ---------------------------------------------------------------------------
// 1. ALL SUPPORTED SHADER TYPES (30 Supported Types)
// ---------------------------------------------------------------------------
export type ShaderType =
  // Effects & Gradients
  | "mesh-gradient"
  | "static-mesh-gradient"
  | "static-radial-gradient"
  | "grain-gradient"
  | "warp"
  | "swirl"
  | "spiral"
  | "god-rays"
  | "pulsing-border"
  | "color-panels"
  | "smoke-ring"
  // Patterns & Particles
  | "dithering"
  | "dot-grid"
  | "dot-orbit"
  | "metaballs"
  // Noise & Waves
  | "neuro-noise"
  | "simplex-noise"
  | "perlin-noise"
  | "voronoi"
  | "waves"
  // Image Filters
  | "paper-texture"
  | "fluted-glass"
  | "water"
  | "image-dithering"
  | "halftone-dots"
  | "halftone-cmyk"
  | "lens-distortion"
  // Logo & Material Effects
  | "liquid-metal"
  | "heatmap"
  | "gem-smoke";

export type ShaderCategory =
  "all" | "effects" | "patterns" | "noise" | "imageFilters" | "logo";

export type ShaderFit = "contain" | "cover";

export type AspectRatioType = "responsive" | "16:9" | "9:16" | "1:1" | "4:3";

// ---------------------------------------------------------------------------
// 2. COMMON SHADER PARAMETERS & SIZING
// ---------------------------------------------------------------------------
export interface CommonSizingParams {
  scale?: number;
  rotation?: number;
  offsetX?: number;
  offsetY?: number;
  fit?: ShaderFit;
  originX?: number;
  originY?: number;
  worldWidth?: number;
  worldHeight?: number;
  minPixelRatio?: number;
  maxPixelCount?: number;
}

export interface CommonMotionParams {
  speed?: number;
  frame?: number;
}

export interface BaseShaderParams
  extends CommonSizingParams, CommonMotionParams {}

// ---------------------------------------------------------------------------
// 3. INDIVIDUAL SHADER PARAMETER SCHEMAS
// ---------------------------------------------------------------------------

export interface MeshGradientParams extends BaseShaderParams {
  colors: string[];
  distortion?: number;
  swirl?: number;
  grainMixer?: number;
  grainOverlay?: number;
}

export interface StaticMeshGradientParams extends CommonSizingParams {
  colors: string[];
  distortionX?: number;
  distortionY?: number;
  grainMixer?: number;
  grainOverlay?: number;
}

export interface StaticRadialGradientParams extends CommonSizingParams {
  colors: string[];
  radius?: number;
  distortion?: number;
  grainMixer?: number;
  grainOverlay?: number;
}

export interface GrainGradientParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  shape?: "circle" | "wave" | "spiral" | "square" | "star" | "flower" | string;
  grain?: number;
  softness?: number;
}

export interface WarpParams extends BaseShaderParams {
  colors: string[];
  shape?: "checks" | "stripes" | "edge" | string;
  shapeScale?: number;
  proportion?: number;
  softness?: number;
  distortion?: number;
  swirl?: number;
  swirlIterations?: number;
}

export interface SwirlParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  swirl?: number;
  density?: number;
}

export interface SpiralParams extends BaseShaderParams {
  colorFront: string;
  colorBack: string;
  density?: number;
  thickness?: number;
  softness?: number;
}

export interface GodRaysParams extends BaseShaderParams {
  colors: string[];
  density?: number;
  glow?: number;
  noise?: number;
}

export interface PulsingBorderParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  thickness?: number;
  glow?: number;
}

export interface ColorPanelsParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  opacity?: number;
}

export interface SmokeRingParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  thickness?: number;
  noise?: number;
}

export interface DitheringParams extends BaseShaderParams {
  colorFront: string;
  colorBack: string;
  shape?:
    | "noise"
    | "warp"
    | "dots"
    | "waves"
    | "ripple"
    | "swirl"
    | "sphere"
    | string;
  type?: "ordered" | "random" | "bayer" | string;
}

export interface DotGridParams extends BaseShaderParams {
  colorFront: string;
  colorBack: string;
  shape?: "circle" | "diamond" | "square" | "triangle" | string;
  size?: number;
  gapX?: number;
  gapY?: number;
}

export interface DotOrbitParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  dotSize?: number;
  orbitRadius?: number;
}

export interface MetaballsParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  count?: number;
  viscosity?: number;
}

export interface NeuroNoiseParams extends BaseShaderParams {
  colorFront: string;
  colorBack: string;
  brightness?: number;
}

export interface SimplexNoiseParams extends BaseShaderParams {
  colors: string[];
  stepsPerColor?: number;
  softness?: number;
}

export interface PerlinNoiseParams extends BaseShaderParams {
  colors: string[];
  octaves?: number;
  lacunarity?: number;
}

export interface VoronoiParams extends BaseShaderParams {
  colors: string[];
  borderWidth?: number;
  borderColor?: string;
}

export interface WavesParams extends BaseShaderParams {
  colorFront: string;
  colorBack: string;
  spacing?: number;
  amplitude?: number;
}

export interface PaperTextureParams extends CommonSizingParams {
  colorBack: string;
  roughness?: number;
  contrast?: number;
  image?: string;
}

export interface FlutedGlassParams extends BaseShaderParams {
  distortion?: number;
  gridShape?: "stripes" | "checks" | "honeycomb" | string;
  image?: string;
}

export interface WaterParams extends BaseShaderParams {
  colorBack?: string;
  distortion?: number;
  image?: string;
}

export interface ImageDitheringParams extends CommonSizingParams {
  colorFront: string;
  colorBack: string;
  type?: "ordered" | "random" | "bayer" | "noise" | string;
  image?: string;
}

export interface HalftoneDotsParams extends BaseShaderParams {
  colorFront: string;
  colorBack: string;
  grid?: "square" | "hexagonal" | string;
  image?: string;
}

export interface HalftoneCmykParams extends BaseShaderParams {
  size?: number;
  softness?: number;
  image?: string;
}

export interface LensDistortionParams extends BaseShaderParams {
  distortion?: number;
  chromaticAberration?: number;
  image?: string;
}

export interface LiquidMetalParams extends BaseShaderParams {
  colorBack: string;
  colorTint: string;
  shape?: "none" | "circle" | "daisy" | "diamond" | "metaballs" | string;
  repetition?: number;
  softness?: number;
  distortion?: number;
  contour?: number;
  angle?: number;
  image?: string;
}

export interface HeatmapParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  shape?: "none" | "circle" | "diamond" | "star" | string;
  image?: string;
}

export interface GemSmokeParams extends BaseShaderParams {
  colors: string[];
  colorBack?: string;
  colorInner?: string;
  shape?: "diamond" | "circle" | "star" | string;
  outerGlow?: number;
  innerGlow?: number;
  innerDistortion?: number;
  outerDistortion?: number;
  image?: string;
}

/**
 * Union map of all shader parameter types
 */
export type AnyShaderParams =
  | MeshGradientParams
  | StaticMeshGradientParams
  | StaticRadialGradientParams
  | GrainGradientParams
  | WarpParams
  | SwirlParams
  | SpiralParams
  | GodRaysParams
  | PulsingBorderParams
  | ColorPanelsParams
  | SmokeRingParams
  | DitheringParams
  | DotGridParams
  | DotOrbitParams
  | MetaballsParams
  | NeuroNoiseParams
  | SimplexNoiseParams
  | PerlinNoiseParams
  | VoronoiParams
  | WavesParams
  | PaperTextureParams
  | FlutedGlassParams
  | WaterParams
  | ImageDitheringParams
  | HalftoneDotsParams
  | HalftoneCmykParams
  | LensDistortionParams
  | LiquidMetalParams
  | HeatmapParams
  | GemSmokeParams;

// ---------------------------------------------------------------------------
// 4. JSON-BASED BACKGROUND CONFIG SCHEMA
// ---------------------------------------------------------------------------
export interface BackgroundConfig<TParams = Record<string, unknown>> {
  version: "1.0";
  id: string;
  name: string;
  shaderType: ShaderType;
  category: ShaderCategory;
  aspectRatio?: AspectRatioType;
  params: TParams;
  createdAt?: string;
  metadata?: {
    tags?: string[];
    author?: string;
    description?: string;
  };
}

// ---------------------------------------------------------------------------
// 5. PRESETS & METADATA TYPES FOR EDITORS
// ---------------------------------------------------------------------------
export interface ShaderPresetDef<TParams = Record<string, unknown>> {
  name: string;
  params: TParams;
}

export interface ParamDescriptor {
  name: string;
  type: "number" | "string" | "string[]" | "enum" | "boolean";
  min?: number;
  max?: number;
  step?: number;
  options?: string[];
  isColor?: boolean;
  defaultValue?: unknown;
  category?: "custom" | "sizing" | "motion" | "color";
}

export interface ShaderRegistryMeta {
  id: ShaderType;
  name: string;
  nameTr: string;
  descriptionTr: string;
  descriptionEn: string;
  category: ShaderCategory;
  presets: ShaderPresetDef[];
  defaultParams: Record<string, unknown>;
  params: ParamDescriptor[];
  supportsImage?: boolean;
}
