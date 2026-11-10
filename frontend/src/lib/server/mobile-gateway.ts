/**
 * Mobile API gateway: `/api/mobile/v1/*` → Go `/v1/*` (TEC-1178, design:
 * docs/design/mobile.md §2.1, KARAR-1). The Go API stays private; the mobile
 * app reaches it through Next.js like the browser does, but this route is
 * separate from the cookie BFF (`bff-proxy.ts`):
 *
 * - No cookies at all. The Auth.js session cookie is never read (no
 *   `auth-tokens` import), and no cookie is ever set: upstream `Set-Cookie` is
 *   dropped by the response-header allowlist. A browser request that only
 *   carries the session cookie reaches Go unauthenticated.
 * - Auth is the client's own `Authorization: Bearer <access>`, forwarded as is.
 *   The app keeps the token pair itself (secure storage), so the login,
 *   refresh, email-code and account-switch responses keep `access_token` /
 *   `refresh_token` in the JSON (the cookie BFF strips them). No refresh is done
 *   here: the app refreshes itself (single flight) on a 401.
 * - The same path checks as the BFF (`isForbiddenProxyPath`), plus routes only a
 *   server or payment provider calls (`isMobileGatewayRefusedPath`): 404.
 * - No CSRF check and no CORS headers: there is no ambient credential to abuse,
 *   and other origins cannot read the responses.
 * - Forwards X-Forwarded-For / User-Agent (`upstream.ts`, so Go resolves the
 *   client IP through TRUSTED_PROXIES) and the app's X-Client / X-App-Version /
 *   X-Platform headers (display + the 426 APP_UPDATE_REQUIRED version gate).
 * - Same body limits as the BFF (`proxy-body.ts`).
 */
import { describeError } from "@/lib/observability/redact";
import {
  bffErrorResponse,
  isMobileGatewayRefusedPath,
} from "@/lib/server/bff-guard";
import {
  isBodyTooLarge,
  passthroughHeaders,
  readBoundedBody,
  streamPassthroughHeaders,
} from "@/lib/server/proxy-body";
import {
  fetchUpstreamStream,
  REQUEST_ID_HEADER,
  resolveRequestId,
  UpstreamUnavailableError,
} from "@/lib/server/upstream";

/** A well-formed Bearer credential (RFC 6750 token68). Anything else is dropped. */
const BEARER_PATTERN = /^Bearer [A-Za-z0-9\-._~+/]+=*$/i;

/** App-identity headers forwarded to Go; values are short printable tokens. */
const CLIENT_HEADERS = ["X-Client", "X-App-Version", "X-Platform"] as const;
const CLIENT_HEADER_PATTERN = /^[A-Za-z0-9._+\- ]{1,64}$/;

const ACCEPT_LANGUAGE_PATTERN = /^[A-Za-z0-9,;=.\-* ]{1,128}$/;

/** Build the upstream request headers: only what the gateway chooses to pass. */
export function mobileUpstreamHeaders(
  request: Request,
  requestId: string,
): Headers {
  const headers = new Headers();
  headers.set(REQUEST_ID_HEADER, requestId);
  headers.set("Accept", request.headers.get("accept") || "application/json");
  const contentType = request.headers.get("content-type");
  if (contentType) headers.set("Content-Type", contentType);

  const authorization = request.headers.get("authorization")?.trim();
  if (authorization && BEARER_PATTERN.test(authorization)) {
    headers.set("Authorization", authorization);
  }
  for (const name of CLIENT_HEADERS) {
    const value = request.headers.get(name)?.trim();
    if (value && CLIENT_HEADER_PATTERN.test(value)) headers.set(name, value);
  }
  const language = request.headers.get("accept-language")?.trim();
  if (language && ACCEPT_LANGUAGE_PATTERN.test(language)) {
    headers.set("Accept-Language", language);
  }
  return headers;
}

async function forward(
  path: string,
  request: Request,
  requestId: string,
): Promise<Response> {
  const url = new URL(request.url);
  const body = await readBoundedBody(request);
  const headers = mobileUpstreamHeaders(request, requestId);

  // Nothing is rewritten, so every response (JSON, file, SSE) is streamed back.
  const result = await fetchUpstreamStream(`${path}${url.search}`, {
    method: request.method,
    headers,
    body:
      body === null || body instanceof ReadableStream
        ? body
        : body.byteLength > 0
          ? body
          : null,
    signal: request.signal,
  });

  const contentType = result.headers.get("content-type") ?? "";
  const responseHeaders = contentType.includes("text/event-stream")
    ? streamPassthroughHeaders(result.headers)
    : passthroughHeaders(result.headers, { keepLength: true });
  return new Response(result.body, {
    status: result.status,
    headers: responseHeaders,
  });
}

/** nginx convention for "client closed the connection before the response". */
const CLIENT_CLOSED_REQUEST = 499;

/**
 * Route handler entry for `/api/mobile/v1/[...path]`. Errors are answered in
 * the API's JSON envelope (404 refused path, 413 body too large, 502 API
 * unreachable, 500 otherwise); a request the client cancelled gets 499.
 */
export async function handleMobileGatewayRequest(
  pathSegments: string[],
  request: Request,
): Promise<Response> {
  const requestId = resolveRequestId(request.headers.get(REQUEST_ID_HEADER));
  if (isMobileGatewayRefusedPath(pathSegments)) {
    return bffErrorResponse(404, "NOT_FOUND", "not found", requestId);
  }
  const path = pathSegments.join("/");
  try {
    return await forward(path, request, requestId);
  } catch (err) {
    if (isBodyTooLarge(err)) {
      return bffErrorResponse(
        413,
        "PAYLOAD_TOO_LARGE",
        "request body too large",
        requestId,
      );
    }
    if (request.signal.aborted) {
      return new Response(null, { status: CLIENT_CLOSED_REQUEST });
    }
    if (err instanceof UpstreamUnavailableError) {
      console.error(
        "[mobile-gateway] upstream unavailable",
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
    console.error("[mobile-gateway]", path, describeError(err));
    return bffErrorResponse(500, "INTERNAL_ERROR", "internal error", requestId);
  }
}
