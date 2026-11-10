/**
 * Request-body limits and response-header filtering shared by the cookie BFF
 * (`bff-proxy.ts`, `/api/v1/*`) and the mobile gateway (`mobile-gateway.ts`,
 * `/api/mobile/v1/*`). No Next or cookie imports, so both proxies and their
 * node tests can load it.
 */

/**
 * Request bodies up to this size are buffered (the cookie BFF can then retry
 * after a 401 refresh). It covers STORAGE_MAX_BYTES (default 10 MiB, the
 * per-file media limit) plus multipart overhead.
 */
export const BUFFERED_BODY_LIMIT = 16 << 20;

/**
 * Larger (or unknown-length) bodies are streamed to Go (`duplex: "half"`)
 * without buffering, capped at the largest upload the API accepts (storage
 * explorer: 512 MiB) plus multipart overhead. Go enforces its own per-route
 * limits on top.
 */
export const STREAMED_BODY_LIMIT = (512 << 20) + (1 << 20);

export class BodyTooLargeError extends Error {}

function declaredLength(request: Request): number | null {
  const raw = request.headers.get("content-length");
  if (raw === null || !/^\d+$/.test(raw.trim())) return null;
  return Number(raw);
}

/** Pass chunks through, erroring once more than `limit` bytes went by. */
function limitStream(
  body: ReadableStream<Uint8Array>,
  limit: number,
): ReadableStream<Uint8Array> {
  let seen = 0;
  return body.pipeThrough(
    new TransformStream<Uint8Array, Uint8Array>({
      transform(chunk, controller) {
        seen += chunk.byteLength;
        if (seen > limit) {
          controller.error(new BodyTooLargeError("request body too large"));
          return;
        }
        controller.enqueue(chunk);
      },
    }),
  );
}

async function readBounded(
  body: ReadableStream<Uint8Array>,
  limit: number,
): Promise<ArrayBuffer> {
  const reader = body.getReader();
  const chunks: Uint8Array[] = [];
  let total = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    total += value.byteLength;
    if (total > limit) {
      await reader.cancel().catch(() => undefined);
      throw new BodyTooLargeError("request body too large");
    }
    chunks.push(value);
  }
  const merged = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    merged.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return merged.buffer;
}

/**
 * Read a client body without ever holding an unbounded amount in memory:
 * small (declared ≤ BUFFERED_BODY_LIMIT) bodies, and every body when
 * `mustBuffer` is set, are buffered with a hard cap; everything else is
 * streamed with the STREAMED_BODY_LIMIT cap. A declared length over the
 * streaming cap is refused before any byte is read.
 */
export async function readBoundedBody(
  request: Request,
  { mustBuffer = false }: { mustBuffer?: boolean } = {},
): Promise<ArrayBuffer | ReadableStream<Uint8Array> | null> {
  if (request.method === "GET" || request.method === "HEAD") return null;
  if (!request.body) return new ArrayBuffer(0);
  const length = declaredLength(request);
  if (length !== null && length > STREAMED_BODY_LIMIT) {
    throw new BodyTooLargeError("request body too large");
  }
  if (mustBuffer || (length !== null && length <= BUFFERED_BODY_LIMIT)) {
    return readBounded(request.body, BUFFERED_BODY_LIMIT);
  }
  return limitStream(request.body, STREAMED_BODY_LIMIT);
}

export function isBodyTooLarge(err: unknown): boolean {
  // A streamed body over the cap surfaces as a fetch error whose cause chain
  // holds our BodyTooLargeError.
  for (let e: unknown = err, i = 0; e && i < 4; i += 1) {
    if (e instanceof BodyTooLargeError) return true;
    e = (e as { cause?: unknown }).cause;
  }
  return false;
}

/**
 * Response headers forwarded to the client. An allowlist: `Set-Cookie` (and
 * every other upstream header) is never passed on. Content-Length is only kept
 * for untouched binary passthrough bodies: buffered bodies may be rewritten
 * (auth token paths strip tokens, so the JSON gets shorter) and fetch()
 * transparently decompresses encoded bodies, so forwarding the upstream length
 * (or Content-Encoding) there makes browsers fail with
 * ERR_CONTENT_LENGTH_MISMATCH or try to decode plain bytes. Without it the
 * runtime sets the correct length (or chunked encoding) itself.
 * Content-Encoding is never forwarded.
 */
export function passthroughHeaders(
  upstream: Headers,
  { keepLength = false }: { keepLength?: boolean } = {},
): Headers {
  const out = new Headers();
  const allow = [
    "content-type",
    "content-disposition",
    "cache-control",
    "location",
    "x-request-id",
    "x-content-type-options",
    "content-security-policy",
  ] as const;
  for (const key of allow) {
    const value = upstream.get(key);
    if (value) out.set(key, value);
  }
  const length = upstream.get("content-length");
  if (keepLength && length && !upstream.get("content-encoding")) {
    out.set("content-length", length);
  }
  return out;
}

/** Headers for a live `text/event-stream` passthrough. */
export function streamPassthroughHeaders(upstream: Headers): Headers {
  const out = passthroughHeaders(upstream);
  if (!out.has("content-type")) {
    out.set("Content-Type", "text/event-stream");
  }
  out.set("Cache-Control", "no-cache, no-transform");
  out.set("Connection", "keep-alive");
  out.set("X-Accel-Buffering", "no");
  return out;
}
