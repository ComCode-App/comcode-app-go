/**
 * Browser-side QR files (TEC-1270): the SVG comes from the shared engine with the server's own
 * matrix, so it equals /qr/render?format=svg; PNG rasterizes that SVG on a canvas. Nothing is
 * sent to the server.
 */

/**
 * Reads API-owned profile media (never a constructed key) into a data: URL so the SVG file
 * stands alone and a canvas stays untainted. Returns null when the browser may not read it
 * (for example a cross-origin bucket without CORS); callers then fall back to server files.
 */
export async function fetchAsDataUrl(
  url: string,
  signal?: AbortSignal,
): Promise<string | null> {
  try {
    const res = await fetch(url, { credentials: "same-origin", signal });
    if (!res.ok) return null;
    const blob = await res.blob();
    if (!/^image\/(png|jpeg)$/.test(blob.type)) return null;
    return await new Promise<string | null>((resolve) => {
      const reader = new FileReader();
      reader.onload = () =>
        resolve(typeof reader.result === "string" ? reader.result : null);
      reader.onerror = () => resolve(null);
      reader.readAsDataURL(blob);
    });
  } catch {
    return null;
  }
}

export function svgDataUrl(svg: string): string {
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
}

export function svgBlob(svg: string): Blob {
  return new Blob([svg], { type: "image/svg+xml" });
}

/** Rasterizes an SVG document to a PNG of size x size pixels. */
export function svgToPngBlob(svg: string, size: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement("canvas");
      canvas.width = size;
      canvas.height = size;
      const ctx = canvas.getContext("2d");
      if (!ctx) {
        reject(new Error("canvas unavailable"));
        return;
      }
      ctx.drawImage(img, 0, 0, size, size);
      canvas.toBlob(
        (blob) => (blob ? resolve(blob) : reject(new Error("PNG failed"))),
        "image/png",
      );
    };
    img.onerror = () => reject(new Error("SVG could not be drawn"));
    img.src = svgDataUrl(svg);
  });
}
