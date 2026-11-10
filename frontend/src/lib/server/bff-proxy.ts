import { describeError } from "@/lib/observability/redact";
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
  BUFFERED_BODY_LIMIT,
  isBodyTooLarge,
  passthroughHeaders,
  readBoundedBody,
  STREAMED_BODY_LIMIT,
  streamPassthroughHeaders,
} from "@/lib/server/proxy-body";
import {
  fetchUpstream,
  fetchUpstreamStream,
  REQUEST_ID_HEADER,
  resolveRequestId,
  UpstreamUnavailableError,
} from "@/lib/server/upstream";

// Re-exported for bff-proxy.test.mjs.
export { BUFFERED_BODY_LIMIT, passthroughHeaders, STREAMED_BODY_LIMIT };

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
 * Read the browser body without ever holding an unbounded amount in memory:
 * small (declared ≤ BUFFERED_BODY_LIMIT) bodies and the auth paths whose body
 * the BFF rewrites are buffered with a hard cap; everything else is streamed.
 */
export async function readRequestBody(
  path: string,
  request: Request,
): Promise<ArrayBuffer | ReadableStream<Uint8Array> | null> {
  return readBoundedBody(request, {
    mustBuffer: isRefreshPath(path) || isLogoutPath(path),
  });
}

/** Refresh before a non-replayable request when the access token is this close to expiry. */
const PROACTIVE_REFRESH_SKEW_S = 30;

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
      console.error(
        "[bff] upstream unavailable",
        path,
        describeError(err.cause ?? err),
      );
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

/** nginx convention for "client closed the connection before the response". */
const CLIENT_CLOSED_REQUEST = 499;

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
    if (request.signal.aborted) {
      // The browser went away mid-request (navigation, closed tab): not a server
      // fault. Logged as 499 at info level; nobody reads the response.
      console.info(
        "[bff] client closed request",
        JSON.stringify({
          status: CLIENT_CLOSED_REQUEST,
          method: request.method,
          path: pathSegments.join("/"),
        }),
      );
      return new Response(null, { status: CLIENT_CLOSED_REQUEST });
    }
    console.error("[bff]", pathSegments.join("/"), describeError(err));
    return bffErrorResponse(
      500,
      "INTERNAL_ERROR",
      "internal error",
      resolveRequestId(request.headers.get(REQUEST_ID_HEADER)),
    );
  }
}
