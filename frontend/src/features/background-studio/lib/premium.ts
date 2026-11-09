import type { ShaderType } from "../data/shader-types";
import tiers from "../data/shader-tiers.json";

/**
 * Premium backgrounds (TEC-947). Solid colors, gradients and static shaders (no motion params)
 * are free; animated shaders and videos need the `design.premium` entitlement. Mirrors
 * `model.IsPremium` / `StaticShaderTypes` in the backend (premium.test.mjs checks the list).
 */

const STATIC_SHADERS = new Set<string>(tiers.static);

export function isPremiumShader(type: ShaderType | string): boolean {
  return !STATIC_SHADERS.has(type);
}

export function isPremiumBackground(
  doc: { kind?: string; shaderType?: string } | null | undefined,
): boolean {
  if (!doc) return false;
  if (doc.kind === "video") return true;
  return (
    doc.kind === "shader" &&
    typeof doc.shaderType === "string" &&
    isPremiumShader(doc.shaderType)
  );
}
