"use client";

import { Component, useEffect, useState, type ReactNode } from "react";

import { cn } from "@/lib/utils";

import type { ShaderType } from "../data/shader-types";
import { SHADER_LOADERS, type ShaderComponent } from "../lib/shader-loaders";

/** Renders nothing (the static fallback underneath stays visible) if the shader throws. */
class ShaderErrorBoundary extends Component<
  { children: ReactNode },
  { failed: boolean }
> {
  state = { failed: false };
  static getDerivedStateFromError() {
    return { failed: true };
  }
  render() {
    return this.state.failed ? null : this.props.children;
  }
}

type LazyShaderProps = {
  shaderType: ShaderType;
  params: Record<string, unknown>;
  /** Freeze on the configured `frame` (speed 0): reduced motion, off-screen or hidden tab. */
  paused?: boolean;
  className?: string;
};

// Phones have DPR 3+; a background does not need native resolution. Caps the fragment work.
const MAX_PIXEL_COUNT = 1280 * 720 * 2;

/**
 * Loads only the selected shader's chunk, then fades it in over whatever static layer sits below.
 * Callers decide *whether* to mount it (device budget, idle, reduced motion); see TEC-600.
 */
export function LazyShader({
  shaderType,
  params,
  paused = false,
  className,
}: LazyShaderProps) {
  const [loaded, setLoaded] = useState<{
    type: ShaderType;
    component: ShaderComponent;
  } | null>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    let cancelled = false;
    SHADER_LOADERS[shaderType]?.()
      .then((component) => {
        if (!cancelled) setLoaded({ type: shaderType, component });
      })
      .catch(() => {
        // chunk failed to load (offline, deploy skew): keep the static fallback
      });
    return () => {
      cancelled = true;
    };
  }, [shaderType]);

  // Fade in one frame after mount so the first WebGL draw is not a hard cut over the fallback.
  useEffect(() => {
    if (!loaded) return;
    const raf = requestAnimationFrame(() => setVisible(true));
    return () => cancelAnimationFrame(raf);
  }, [loaded]);

  if (!loaded || loaded.type !== shaderType) return null;
  const Shader = loaded.component;
  return (
    <div
      className={cn(
        "absolute inset-0 transition-opacity duration-700",
        visible ? "opacity-100" : "opacity-0",
        className,
      )}
    >
      <ShaderErrorBoundary>
        <Shader
          {...params}
          {...(paused ? { speed: 0 } : {})}
          minPixelRatio={1}
          maxPixelCount={MAX_PIXEL_COUNT}
          style={{ width: "100%", height: "100%", display: "block" }}
        />
      </ShaderErrorBoundary>
    </div>
  );
}
