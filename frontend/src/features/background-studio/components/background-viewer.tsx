"use client";

import type { CSSProperties } from "react";

import { cn } from "@/lib/utils";

import { SHADER_REGISTRY } from "../data/shaders-registry";
import { aspectRatioClass } from "../lib/defaults";
import { shaderFallbackStyle } from "../lib/shader-fallback";
import type { BackgroundDocument } from "../types";

type BackgroundViewerProps = {
  doc: BackgroundDocument;
  className?: string;
  style?: CSSProperties;
  fullscreen?: boolean;
  paused?: boolean;
};

function gradientCss(doc: Extract<BackgroundDocument, { kind: "gradient" }>) {
  const { type, colors, angle = 135 } = doc.params;
  const stops = colors.join(", ");
  if (type === "radial") {
    return `radial-gradient(circle at center, ${stops})`;
  }
  return `linear-gradient(${angle}deg, ${stops})`;
}

export function BackgroundViewer({
  doc,
  className,
  style,
  fullscreen = false,
  paused = false,
}: BackgroundViewerProps) {
  const frameClass = cn(
    "relative overflow-hidden rounded-xl border bg-muted",
    aspectRatioClass(doc.aspectRatio, fullscreen),
    className,
  );

  if (doc.kind === "solid") {
    return (
      <div
        className={frameClass}
        style={{ backgroundColor: doc.params.color, ...style }}
      />
    );
  }

  if (doc.kind === "gradient") {
    return (
      <div
        className={frameClass}
        style={{ backgroundImage: gradientCss(doc), ...style }}
      />
    );
  }

  if (doc.kind === "video") {
    const { src, poster, muted, loop, autoplay, objectFit } = doc.params;
    return (
      <div className={frameClass} style={style}>
        {src ? (
          <video
            key={src}
            className="h-full w-full"
            style={{ objectFit: objectFit ?? "cover" }}
            src={src}
            poster={poster}
            muted={muted !== false}
            loop={loop !== false}
            autoPlay={autoplay !== false && !paused}
            playsInline
            controls={false}
          />
        ) : (
          <div className="flex h-full min-h-[240px] items-center justify-center text-sm text-muted-foreground">
            —
          </div>
        )}
      </div>
    );
  }

  // shader
  const entry = SHADER_REGISTRY[doc.shaderType];
  if (!entry) {
    return (
      <div className={cn(frameClass, "flex items-center justify-center")} style={style}>
        <span className="text-sm text-muted-foreground">{doc.shaderType}</span>
      </div>
    );
  }

  const Component = entry.component;
  const params = {
    ...doc.params,
    ...(paused ? { speed: 0 } : {}),
  };

  // The palette fallback sits under the canvas: visible while WebGL initialises or if it fails.
  return (
    <div className={frameClass} style={{ ...shaderFallbackStyle(doc.params), ...style }}>
      <Component
        {...params}
        style={{ width: "100%", height: "100%", display: "block" }}
      />
    </div>
  );
}
