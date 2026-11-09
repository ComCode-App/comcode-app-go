import { SHADER_REGISTRY } from "../data/shaders-registry";
import type {
  BackgroundDocument,
  BackgroundKind,
  LegacyShaderConfig,
  ShaderType,
} from "../types";
import {
  createGradientBackground,
  createShaderBackground,
  createSolidBackground,
  createVideoBackground,
} from "./defaults";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isKind(value: unknown): value is BackgroundKind {
  return (
    value === "solid" ||
    value === "gradient" ||
    value === "video" ||
    value === "shader"
  );
}

function pickMeta(raw: Record<string, unknown>) {
  return {
    ...(typeof raw.id === "string" ? { id: raw.id } : {}),
    ...(typeof raw.name === "string" ? { name: raw.name } : {}),
    ...(typeof raw.aspectRatio === "string"
      ? { aspectRatio: raw.aspectRatio as BackgroundDocument["aspectRatio"] }
      : {}),
    ...(typeof raw.createdAt === "string" ? { createdAt: raw.createdAt } : {}),
    ...(isRecord(raw.metadata)
      ? { metadata: raw.metadata as BackgroundDocument["metadata"] }
      : {}),
  };
}

/** Accepts unified documents or legacy furkan shader-only JSON. */
export function parseBackgroundDocument(raw: unknown): BackgroundDocument {
  if (!isRecord(raw)) {
    throw new Error("invalid_json");
  }

  if (isKind(raw.kind)) {
    switch (raw.kind) {
      case "solid": {
        const params = isRecord(raw.params) ? raw.params : {};
        const color =
          typeof params.color === "string" ? params.color : "#0f172a";
        return createSolidBackground({
          ...pickMeta(raw),
          params: { color },
        });
      }
      case "gradient": {
        const params = isRecord(raw.params) ? raw.params : {};
        const colors = Array.isArray(params.colors)
          ? params.colors.filter((c): c is string => typeof c === "string")
          : ["#e0eaff", "#241d9a"];
        return createGradientBackground({
          ...pickMeta(raw),
          params: {
            type: params.type === "radial" ? "radial" : "linear",
            colors: colors.length >= 2 ? colors : ["#e0eaff", "#241d9a"],
            angle: typeof params.angle === "number" ? params.angle : 135,
          },
        });
      }
      case "video": {
        const params = isRecord(raw.params) ? raw.params : {};
        return createVideoBackground({
          ...pickMeta(raw),
          params: {
            src: typeof params.src === "string" ? params.src : "",
            ...(typeof params.poster === "string"
              ? { poster: params.poster }
              : {}),
            muted: params.muted !== false,
            loop: params.loop !== false,
            autoplay: params.autoplay !== false,
            objectFit:
              params.objectFit === "contain" || params.objectFit === "fill"
                ? params.objectFit
                : "cover",
          },
        });
      }
      case "shader": {
        const shaderType =
          typeof raw.shaderType === "string" && SHADER_REGISTRY[raw.shaderType]
            ? (raw.shaderType as ShaderType)
            : "mesh-gradient";
        const entry = SHADER_REGISTRY[shaderType];
        return createShaderBackground(shaderType, {
          ...pickMeta(raw),
          category: entry.category,
          params: isRecord(raw.params)
            ? { ...entry.defaultParams, ...raw.params }
            : { ...entry.defaultParams },
        });
      }
    }
  }

  // Legacy furkan BackgroundConfig
  const legacy = raw as unknown as LegacyShaderConfig;
  if (
    typeof legacy.shaderType === "string" &&
    SHADER_REGISTRY[legacy.shaderType]
  ) {
    const shaderType = legacy.shaderType as ShaderType;
    const entry = SHADER_REGISTRY[shaderType];
    return createShaderBackground(shaderType, {
      ...(typeof legacy.id === "string" ? { id: legacy.id } : {}),
      ...(typeof legacy.name === "string" ? { name: legacy.name } : {}),
      ...(legacy.aspectRatio ? { aspectRatio: legacy.aspectRatio } : {}),
      ...(typeof legacy.createdAt === "string"
        ? { createdAt: legacy.createdAt }
        : {}),
      ...(legacy.metadata ? { metadata: legacy.metadata } : {}),
      category: entry.category,
      params: { ...entry.defaultParams, ...(legacy.params ?? {}) },
    });
  }

  throw new Error("invalid_json");
}

export function stringifyBackground(doc: BackgroundDocument): string {
  return JSON.stringify(doc, null, 2);
}

export function downloadJson(doc: BackgroundDocument, filename?: string) {
  const blob = new Blob([stringifyBackground(doc)], {
    type: "application/json",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename ?? `${doc.id || "background"}.json`;
  a.click();
  URL.revokeObjectURL(url);
}
