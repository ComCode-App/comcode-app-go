"use client";

import {
  useEffect,
  useState,
  useSyncExternalStore,
  type RefObject,
} from "react";

/**
 * Device / preference gates for shader backgrounds (TEC-600). Public profiles are mostly opened via
 * NFC/QR on low-mid phones, so WebGL is opt-in on capable devices only.
 */

type NavigatorWithHints = Navigator & {
  deviceMemory?: number;
  connection?: { saveData?: boolean };
};

/** Weak-device heuristics: data saver, ≤2 logical cores or ≤2 GB RAM (Chromium-only hint). */
export function isConstrainedDevice(): boolean {
  if (typeof navigator === "undefined") return true;
  const nav = navigator as NavigatorWithHints;
  if (nav.connection?.saveData) return true;
  if (
    typeof nav.hardwareConcurrency === "number" &&
    nav.hardwareConcurrency > 0 &&
    nav.hardwareConcurrency <= 2
  ) {
    return true;
  }
  if (typeof nav.deviceMemory === "number" && nav.deviceMemory <= 2)
    return true;
  return false;
}

let webglProbe: boolean | undefined;

/**
 * Whether a hardware-accelerated WebGL2 context can be created (Paper shaders need WebGL2).
 * `failIfMajorPerformanceCaveat` rejects software rasterizers (SwiftShader & co.). The probe context
 * is released right away so it does not count against the browser's context limit.
 */
export function supportsFastWebGL2(): boolean {
  if (webglProbe !== undefined) return webglProbe;
  try {
    const canvas = document.createElement("canvas");
    const gl = canvas.getContext("webgl2", {
      failIfMajorPerformanceCaveat: true,
    });
    webglProbe = !!gl;
    gl?.getExtension("WEBGL_lose_context")?.loseContext();
  } catch {
    webglProbe = false;
  }
  return webglProbe;
}

const REDUCED_MOTION = "(prefers-reduced-motion: reduce)";

function subscribeReducedMotion(onChange: () => void) {
  const query = window.matchMedia(REDUCED_MOTION);
  query.addEventListener("change", onChange);
  return () => query.removeEventListener("change", onChange);
}

/** Live `prefers-reduced-motion`; `true` during SSR/hydration so nothing starts moving early. */
export function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(
    subscribeReducedMotion,
    () => window.matchMedia(REDUCED_MOTION).matches,
    () => true,
  );
}

/**
 * Evaluates `compute` once the browser is idle after the first paint (setTimeout fallback for
 * Safari) and returns its result; `null` until then, and on the server.
 */
export function useIdleValue(
  compute: () => boolean,
  timeoutMs = 2000,
): boolean | null {
  const [result, setResult] = useState<boolean | null>(null);
  useEffect(() => {
    const run = () => setResult(compute());
    if (typeof window.requestIdleCallback === "function") {
      const id = window.requestIdleCallback(run, { timeout: timeoutMs });
      return () => window.cancelIdleCallback(id);
    }
    // A frame plus a beat guarantees a paint happened before the heavy work starts.
    let timer: ReturnType<typeof setTimeout> | undefined;
    const raf = requestAnimationFrame(() => {
      timer = setTimeout(run, 200);
    });
    return () => {
      cancelAnimationFrame(raf);
      if (timer) clearTimeout(timer);
    };
  }, [compute, timeoutMs]);
  return result;
}

/** Can this device afford a WebGL background? Pass to `useIdleValue`. */
export function canRenderShaderBackground(): boolean {
  return !isConstrainedDevice() && supportsFastWebGL2();
}

const noopSubscribe = () => () => {};

/** `isConstrainedDevice()` on the client; `true` during SSR/hydration (cheapest markup first). */
export function useConstrainedDevice(): boolean {
  return useSyncExternalStore(noopSubscribe, isConstrainedDevice, () => true);
}

/** True while the tab is visible and the element intersects the viewport. */
export function useOnScreen(ref: RefObject<Element | null>): boolean {
  const [pageVisible, setPageVisible] = useState(true);
  const [intersecting, setIntersecting] = useState(true);
  useEffect(() => {
    const update = () => setPageVisible(document.visibilityState !== "hidden");
    update();
    document.addEventListener("visibilitychange", update);
    return () => document.removeEventListener("visibilitychange", update);
  }, []);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(([entry]) =>
      setIntersecting(entry?.isIntersecting ?? true),
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, [ref]);
  return pageVisible && intersecting;
}
