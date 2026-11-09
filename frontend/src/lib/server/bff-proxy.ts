import {
  clearAuthSessionCookie,
  getApiTokens,
  persistApiTokens,
} from "@/lib/server/auth-tokens";
import {
  allowedAppOrigins,
  bffErrorResponse,
  isCrossSiteMutation,
  isCsrfExemptPath,
  isForbiddenProxyPath,
  jwtExpiresAt,
} from "@/lib/server/bff-guard";
import {
  sharedRefresh,
  type RefreshResult,
} from "@/lib/server/refresh-single-flight";
import {
  fetchUpstream,
  fetchUpstreamStream,
  REQUEST_ID_HEADER,
  resolveRequestId,
  UpstreamUnavailableError,
} from "@/lib/server/upstream";

type TokensData = {
  access_token?: string;
  refresh_token?: string;
  expires_in?: number;
  access_expires_at?: string;
};

type Envelope = {
  success?: boolean;
  data?: TokensData & Record<string, unknown>;
  meta?: unknown;
  error?: unknown;
};

type SessionSwitchData = {
  user_uuid?: string;
  email?: string;
  impersonator_uuid?: string | null;
};

/** Paths whose successful response updates the NextAuth JWT token pair. */
function isAuthTokenPath(path: string) {
  if (path === "auth/refresh") return true;
  if (path === "auth/impersonation/stop") return true;
  if (path === "auth/accounts/switch") return true;
  return path.startsWith("platform/users/") && path.endsWith("/impersonate");
}

/** Public auth endpoints that must not send Bearer. */
const AUTH_PUBLIC_TOKEN_PATHS = new Set(["auth/refresh", "auth/register"]);

function isAuthPublicTokenPath(path: string) {
  return (
    AUTH_PUBLIC_TOKEN_PATHS.has(path) || path.startsWith("public/payment/")
  );
}

function isLogoutPath(path: string) {
  return path === "auth/logout";
}

function isSessionInvalidatePath(path: string) {
  return path === "auth/password/change" || path === "auth/password/reset";
}

function isRefreshPath(path: string) {
  return path === "auth/refresh";
}

function wantsBinaryStream(path: string) {
  return (
    path.includes("platform/storage/objects/download") ||
    path.includes("platform/storage/objects/preview") ||
    path.startsWith("public/storage/")
  );
}

function wantsEventStream(request: Request) {
  const accept = request.headers.get("accept") ?? "";
  return accept.includes("text/event-stream");
}

function encodeJson(value: unknown): string {
  return JSON.stringify(value);
}

function parseJson(buffer: ArrayBuffer): Envelope | null {
  try {
    const text = new TextDecoder().decode(buffer);
    if (!text) return null;
    return JSON.parse(text) as Envelope;
  } catch {
    return null;
  }
}

function stripTokens(envelope: Envelope): Envelope {
  if (!envelope.data || typeof envelope.data !== "object") return envelope;
  const data = { ...envelope.data };
  delete data.access_token;
  delete data.refresh_token;
  return {
    ...envelope,
    data: {
      ...data,
      authenticated: true,
    },
  };
}

function resolveAccessMaxAge(data: TokensData | undefined): number | undefined {
  if (typeof data?.expires_in === "number" && data.expires_in > 0) {
    return data.expires_in;
  }
  return undefined;
}

async function persistTokensFromEnvelope(envelope: Envelope | null) {
  const access = envelope?.data?.access_token;
  const refresh = envelope?.data?.refresh_token;
  if (!access || !refresh) return false;

  const sessionPayload = envelope?.data?.session as
    SessionSwitchData | undefined;
  const existing = await getApiTokens();
  const userId = sessionPayload?.user_uuid ?? existing.userId;
  if (!userId) return false;

  const impersonatorUuid =
    sessionPayload !== undefined
      ? (sessionPayload.impersonator_uuid ?? null)
      : existing.impersonatorUuid;

  await persistApiTokens({
    accessToken: access,
    refreshToken: refresh,
    userId,
    email: sessionPayload?.email ?? existing.email,
    impersonatorUuid,
    expiresIn: resolveAccessMaxAge(envelope?.data),
  });
  return true;
}

/** One upstream rotation per refresh token, shared by concurrent requests. */
function rotateRefreshToken(refreshToken: string): Promise<RefreshResult> {
  return sharedRefresh(refreshToken, async (token) => {
    const result = await fetchUpstream("auth/refresh", {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
      body: encodeJson({ refresh_token: token }),
    });
    return { status: result.status, body: result.body };
  });
}

async function refreshViaUpstream(): Promise<boolean> {
  const { refreshToken, userId } = await getApiTokens();
  if (!refreshToken || !userId) return false;

  // Never clear the session cookie here: a request that loses a refresh race
  // may see a failure while a concurrent response has already written the
  // rotated pair. The browser's own refresh flow (lib/api/client.ts) signs the
  // user out when the session is really gone.
  const result = await rotateRefreshToken(refreshToken);
  if (result.status >= 400) return false;
  return persistTokensFromEnvelope(parseJson(result.body));
}

function buildUpstreamBody(
  path: string,
  method: string,
  incoming: ArrayBuffer | ReadableStream<Uint8Array> | null,
  refreshToken: string | null,
): BodyInit | null {
  if (method === "GET" || method === "HEAD") return null;
  // Large uploads are streamed through untouched (see readRequestBody).
  if (incoming === null || incoming instanceof ReadableStream) return incoming;

  if (isRefreshPath(path)) {
    return encodeJson({ refresh_token: refreshToken ?? "" });
  }

  if (isLogoutPath(path)) {
    let clientBody: Record<string, unknown> = {};
    try {
      const text = new TextDecoder().decode(incoming);
      if (text) clientBody = JSON.parse(text) as Record<string, unknown>;
    } catch {
      clientBody = {};
    }
    return encodeJson({
      ...clientBody,
      refresh_token: refreshToken ?? clientBody.refresh_token ?? "",
    });
  }

  return incoming.byteLength > 0 ? incoming : null;
}

/**
 * Response headers forwarded to the browser. Content-Length is only kept for
 * untouched binary passthrough bodies: buffered bodies may be rewritten (auth
 * token paths strip tokens, so the JSON gets shorter) and fetch() transparently
 * decompresses encoded bodies, so forwarding the upstream length (or
 * Content-Encoding) there makes browsers fail with ERR_CONTENT_LENGTH_MISMATCH
 * or try to decode plain bytes. Without it the runtime sets the correct length
 * (or chunked encoding) itself. Content-Encoding is never forwarded.
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

function streamPassthroughHeaders(upstream: Headers): Headers {
  const out = passthroughHeaders(upstream);
  if (!out.has("content-type")) {
    out.set("Content-Type", "text/event-stream");
  }
  out.set("Cache-Control", "no-cache, no-transform");
  out.set("Connection", "keep-alive");
  out.set("X-Accel-Buffering", "no");
  return out;
}

/**
 * Request bodies up to this size are buffered so a 401 can be retried after a
 * refresh. It covers STORAGE_MAX_BYTES (default 10 MiB, the per-file media
 * limit) plus multipart overhead, so ordinary uploads keep the retry.
 */
export const BUFFERED_BODY_LIMIT = 16 << 20;

/**
 * Larger (or unknown-length) bodies are streamed to Go (`duplex: "half"`)
 * without buffering, capped at the largest upload the API accepts (storage
 * explorer: 512 MiB) plus multipart overhead. Go enforces its own per-route
 * limits on top.
 */
export const STREAMED_BODY_LIMIT = (512 << 20) + (1 << 20);

/** Refresh before a non-replayable request when the access token is this close to expiry. */
const PROACTIVE_REFRESH_SKEW_S = 30;

class BodyTooLargeError extends Error {}

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
 * Read the browser body without ever holding an unbounded amount in memory:
 * small (declared ≤ BUFFERED_BODY_LIMIT) bodies and the auth paths whose body
 * the BFF rewrites are buffered with a hard cap; everything else is streamed.
 */
export async function readRequestBody(
  path: string,
  request: Request,
): Promise<ArrayBuffer | ReadableStream<Uint8Array> | null> {
  if (request.method === "GET" || request.method === "HEAD") return null;
  if (!request.body) return new ArrayBuffer(0);
  const length = declaredLength(request);
  if (length !== null && length > STREAMED_BODY_LIMIT) {
    throw new BodyTooLargeError("request body too large");
  }
  const mustBuffer = isRefreshPath(path) || isLogoutPath(path);
  if (mustBuffer || (length !== null && length <= BUFFERED_BODY_LIMIT)) {
    return readBounded(request.body, BUFFERED_BODY_LIMIT);
  }
  return limitStream(request.body, STREAMED_BODY_LIMIT);
}

function isReplayable(body: BodyInit | null): boolean {
  return !(body instanceof ReadableStream);
}

async function proxyBuffered(
  path: string,
  pathWithQuery: string,
  request: Request,
  headers: Headers,
  body: BodyInit | null,
  refreshToken: string | null,
): Promise<Response> {
  let result =
    isRefreshPath(path) && refreshToken
      ? // Share the rotation with any BFF-side refresh running for the same token.
        {
          ...(await rotateRefreshToken(refreshToken)),
          headers: new Headers({ "Content-Type": "application/json" }),
        }
      : await fetchUpstream(pathWithQuery, {
          method: request.method,
          headers,
          body,
        });

  if (
    result.status === 401 &&
    Boolean(refreshToken) &&
    isReplayable(body) &&
    !isAuthPublicTokenPath(path) &&
    !isLogoutPath(path)
  ) {
    const refreshed = await refreshViaUpstream();
    if (refreshed) {
      const { accessToken: nextAccess } = await getApiTokens();
      if (nextAccess) headers.set("Authorization", `Bearer ${nextAccess}`);
      result = await fetchUpstream(pathWithQuery, {
        method: request.method,
        headers,
        body,
      });
    }
  }

  if (
    isLogoutPath(path) ||
    (result.status < 400 && isSessionInvalidatePath(path))
  ) {
    await clearAuthSessionCookie();
  }

  // Copy: a shared refresh result is handed to several requests.
  let responseBody: ArrayBuffer | string = result.body.slice(0);
  const envelope = parseJson(result.body);

  if (result.status < 400 && isAuthTokenPath(path)) {
    await persistTokensFromEnvelope(envelope);
    if (envelope) {
      responseBody = JSON.stringify(stripTokens(envelope));
    }
  }

  // The body may have been rewritten (tokens stripped) and fetch() decoded any
  // Content-Encoding, so the upstream length is wrong: let Response derive it.
  return new Response(responseBody, {
    status: result.status,
    headers: passthroughHeaders(result.headers),
  });
}

async function proxyStream(
  path: string,
  pathWithQuery: string,
  request: Request,
  headers: Headers,
  body: BodyInit | null,
  refreshToken: string | null,
): Promise<Response> {
  const binary = wantsBinaryStream(path);
  if (!binary && !headers.get("Accept")?.includes("text/event-stream")) {
    headers.set("Accept", "text/event-stream");
  }

  let result = await fetchUpstreamStream(pathWithQuery, {
    method: request.method,
    headers,
    body,
    signal: request.signal,
  });

  if (
    result.status === 401 &&
    Boolean(refreshToken) &&
    isReplayable(body) &&
    !isAuthPublicTokenPath(path)
  ) {
    await result.body?.cancel().catch(() => undefined);
    const refreshed = await refreshViaUpstream();
    if (refreshed) {
      const { accessToken: nextAccess } = await getApiTokens();
      if (nextAccess) headers.set("Authorization", `Bearer ${nextAccess}`);
      result = await fetchUpstreamStream(pathWithQuery, {
        method: request.method,
        headers,
        body,
        signal: request.signal,
      });
    }
  }

  const contentType = result.headers.get("content-type") ?? "";
  if (binary || !contentType.includes("text/event-stream")) {
    if (binary) {
      // Untouched passthrough: the length is still right unless fetch decoded it.
      return new Response(result.body, {
        status: result.status,
        headers: passthroughHeaders(result.headers, { keepLength: true }),
      });
    }
    const chunks: Uint8Array[] = [];
    if (result.body) {
      const reader = result.body.getReader();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value) chunks.push(value);
      }
    }
    const total = chunks.reduce((n, c) => n + c.byteLength, 0);
    const merged = new Uint8Array(total);
    let offset = 0;
    for (const chunk of chunks) {
      merged.set(chunk, offset);
      offset += chunk.byteLength;
    }
    return new Response(merged, {
      status: result.status,
      headers: passthroughHeaders(result.headers),
    });
  }

  return new Response(result.body, {
    status: result.status,
    headers: streamPassthroughHeaders(result.headers),
  });
}

export async function proxyToUpstream(
  pathSegments: string[],
  request: Request,
): Promise<Response> {
  const requestId = resolveRequestId(request.headers.get(REQUEST_ID_HEADER));

  if (isForbiddenProxyPath(pathSegments)) {
    return bffErrorResponse(404, "NOT_FOUND", "not found", requestId);
  }
  const path = pathSegments.join("/");
  if (
    !isCsrfExemptPath(path) &&
    isCrossSiteMutation(request, allowedAppOrigins(request.headers))
  ) {
    return bffErrorResponse(
      403,
      "FORBIDDEN",
      "cross-site request blocked",
      requestId,
    );
  }

  try {
    return await forwardToUpstream(path, request, requestId);
  } catch (err) {
    if (isBodyTooLarge(err)) {
      return bffErrorResponse(
        413,
        "PAYLOAD_TOO_LARGE",
        "request body too large",
        requestId,
      );
    }
    if (err instanceof UpstreamUnavailableError) {
      console.error("[bff] upstream unavailable", path, err.cause ?? err);
      return bffErrorResponse(
        502,
        "UPSTREAM_UNAVAILABLE",
        "API is unavailable",
        requestId,
      );
    }
    throw err;
  }
}

function isBodyTooLarge(err: unknown): boolean {
  // A streamed body over the cap surfaces as a fetch error whose cause chain
  // holds our BodyTooLargeError.
  for (let e: unknown = err, i = 0; e && i < 4; i += 1) {
    if (e instanceof BodyTooLargeError) return true;
    e = (e as { cause?: unknown }).cause;
  }
  return false;
}

async function forwardToUpstream(
  path: string,
  request: Request,
  requestId: string,
): Promise<Response> {
  const url = new URL(request.url);
  const pathWithQuery = `${path}${url.search}`;

  const incomingBody = await readRequestBody(path, request);

  const tokens = await getApiTokens();
  let { accessToken } = tokens;
  const { refreshToken } = tokens;

  // A streamed body cannot be replayed after a 401, so refresh first when the
  // access token is (about to be) expired.
  if (
    incomingBody instanceof ReadableStream &&
    accessToken &&
    refreshToken &&
    !isAuthPublicTokenPath(path)
  ) {
    const exp = jwtExpiresAt(accessToken);
    if (exp !== null && exp - PROACTIVE_REFRESH_SKEW_S <= Date.now() / 1000) {
      if (await refreshViaUpstream()) {
        accessToken = (await getApiTokens()).accessToken;
      }
    }
  }

  const headers = new Headers();
  // One correlation id per browser request (shared by the 401 refresh retry).
  headers.set(REQUEST_ID_HEADER, requestId);
  const accept = request.headers.get("accept");
  const contentType = request.headers.get("content-type");
  if (accept) headers.set("Accept", accept);
  else headers.set("Accept", "application/json");
  if (contentType) headers.set("Content-Type", contentType);

  if (accessToken && !isAuthPublicTokenPath(path)) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  const body = buildUpstreamBody(
    path,
    request.method,
    incomingBody,
    refreshToken,
  );

  if (wantsEventStream(request) || wantsBinaryStream(path)) {
    return proxyStream(
      path,
      pathWithQuery,
      request,
      headers,
      body,
      refreshToken,
    );
  }
  return proxyBuffered(
    path,
    pathWithQuery,
    request,
    headers,
    body,
    refreshToken,
  );
}

/**
 * Route handler entry: never let an exception reset the connection (the edge
 * proxy would report "upstream prematurely closed"); answer with the API's
 * JSON error envelope instead.
 */
export async function handleProxyRequest(
  pathSegments: string[],
  request: Request,
): Promise<Response> {
  try {
    return await proxyToUpstream(pathSegments, request);
  } catch (err) {
    console.error("[bff]", pathSegments.join("/"), err);
    return bffErrorResponse(
      500,
      "INTERNAL_ERROR",
      "internal error",
      resolveRequestId(request.headers.get(REQUEST_ID_HEADER)),
    );
  }
}
