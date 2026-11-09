"use client";

import { useMemo, useRef, type CSSProperties } from "react";

import { LazyShader } from "@/features/background-studio/components/lazy-shader";
import type { ShaderType } from "@/features/background-studio/data/shader-types";
import {
  canRenderShaderBackground,
  useConstrainedDevice,
  useIdleValue,
  useOnScreen,
  usePrefersReducedMotion,
} from "@/features/background-studio/lib/render-budget";
import { shaderFallbackStyle } from "@/features/background-studio/lib/shader-fallback";
import { isKnownShaderType } from "@/features/background-studio/lib/shader-loaders";
import { assertServiceMediaURL } from "@/lib/media/urls";

/**
 * Background Studio design behind a public profile (TEC-600 budget):
 * - the static layer (solid / gradient / palette derived from the shader) is server-rendered, so it
 *   paints with the HTML and never blocks LCP;
 * - the WebGL shader is mounted only on capable devices (fast WebGL2, no data saver, >2 cores and
 *   >2 GB), after the browser goes idle, and only that shader's chunk is downloaded;
 * - other devices get the design's static thumbnail (TEC-599, rendered by the API on save) over
 *   the CSS layer, so they still see the design's look rather than only its palette;
 * - reduced motion freezes it on its configured frame; it pauses while off-screen or the tab hides.
 *
 * Deliberately avoids `shaders-registry` / `parseBackgroundDocument` (they pull all 30 shaders).
 * The API validated the document on save; this only guards shape.
 */

type PublicBackground =
  | { kind: "solid"; color: string }
  | { kind: "gradient"; css: string }
  | {
      kind: "video";
      src: string;
      poster?: string;
      objectFit: CSSProperties["objectFit"];
      autoplay: boolean;
      loop: boolean;
      muted: boolean;
    }
  | { kind: "shader"; shaderType: ShaderType; params: Record<string, unknown> };

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function normalize(raw: unknown): PublicBackground | null {
  if (!isRecord(raw)) return null;
  const params = isRecord(raw.params) ? raw.params : {};
  // Legacy shader-only documents have no `kind`.
  const kind =
    raw.kind ?? (typeof raw.shaderType === "string" ? "shader" : undefined);
  switch (kind) {
    case "solid":
      return {
        kind: "solid",
        color: typeof params.color === "string" ? params.color : "#0f172a",
      };
    case "gradient": {
      const colors = Array.isArray(params.colors)
        ? params.colors.filter((c): c is string => typeof c === "string")
        : [];
      const stops = (colors.length >= 2 ? colors : ["#e0eaff", "#241d9a"]).join(
        ", ",
      );
      const angle = typeof params.angle === "number" ? params.angle : 135;
      return {
        kind: "gradient",
        css:
          params.type === "radial"
            ? `radial-gradient(circle at center, ${stops})`
            : `linear-gradient(${angle}deg, ${stops})`,
      };
    }
    case "video":
      if (typeof params.src !== "string" || !params.src) return null;
      return {
        kind: "video",
        src: params.src,
        poster: typeof params.poster === "string" ? params.poster : undefined,
        objectFit:
          params.objectFit === "contain" || params.objectFit === "fill"
            ? params.objectFit
            : "cover",
        autoplay: params.autoplay !== false,
        loop: params.loop !== false,
        muted: params.muted !== false,
      };
    case "shader":
      if (!isKnownShaderType(raw.shaderType)) return null;
      return { kind: "shader", shaderType: raw.shaderType, params };
    default:
      return null;
  }
}

const LAYER = "pointer-events-none fixed inset-0 -z-0 overflow-hidden";

export function PublicProfileBackground({
  document,
  thumbnailUrl,
}: {
  document: unknown;
  /** Static stand-in image from the API (`background_thumbnail_url`). */
  thumbnailUrl?: string;
}) {
  const bg = useMemo(() => normalize(document), [document]);
  if (!bg) return null;
  switch (bg.kind) {
    case "solid":
      return (
        <div
          aria-hidden
          className={LAYER}
          style={{ backgroundColor: bg.color }}
        />
      );
    case "gradient":
      return (
        <div
          aria-hidden
          className={LAYER}
          style={{ backgroundImage: bg.css }}
        />
      );
    case "video":
      return <VideoBackground bg={bg} />;
    case "shader":
      return (
        <ShaderBackground
          shaderType={bg.shaderType}
          params={bg.params}
          thumbnailUrl={thumbnailUrl}
        />
      );
  }
}

function VideoBackground({
  bg,
}: {
  bg: Extract<PublicBackground, { kind: "video" }>;
}) {
  const reduceMotion = usePrefersReducedMotion();
  // Data saver / weak devices get the poster only (SSR + hydration render the poster too).
  const constrained = useConstrainedDevice();
  return (
    <div aria-hidden className={LAYER}>
      {!constrained ? (
        <video
          key={bg.src}
          className="h-full w-full"
          style={{ objectFit: bg.objectFit }}
          src={bg.src}
          poster={bg.poster}
          muted={bg.muted}
          loop={bg.loop}
          autoPlay={bg.autoplay && !reduceMotion}
          preload={bg.autoplay && !reduceMotion ? "auto" : "metadata"}
          playsInline
          controls={false}
        />
      ) : bg.poster ? (
        // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, decorative
        <img
          src={bg.poster}
          alt=""
          className="h-full w-full"
          style={{ objectFit: bg.objectFit }}
        />
      ) : null}
    </div>
  );
}

function ShaderBackground({
  shaderType,
  params,
  thumbnailUrl,
}: {
  shaderType: ShaderType;
  params: Record<string, unknown>;
  thumbnailUrl?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const reduceMotion = usePrefersReducedMotion();
  const onScreen = useOnScreen(ref);
  // null until the idle check ran: the CSS layer alone paints first on every device.
  const capable = useIdleValue(canRenderShaderBackground);
  const fallback = useMemo(() => shaderFallbackStyle(params), [params]);
  const thumbnail = capable === false ? safeMediaURL(thumbnailUrl) : null;

  return (
    <div ref={ref} aria-hidden className={LAYER} style={fallback}>
      {capable ? (
        <LazyShader
          shaderType={shaderType}
          params={params}
          paused={reduceMotion || !onScreen}
        />
      ) : null}
      {thumbnail ? (
        // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, decorative
        <img
          src={thumbnail}
          alt=""
          decoding="async"
          className="h-full w-full object-cover"
        />
      ) : null}
    </div>
  );
}

function safeMediaURL(url: string | undefined): string | null {
  try {
    return assertServiceMediaURL(url);
  } catch {
    return null;
  }
}
