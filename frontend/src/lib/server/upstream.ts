import { headers as requestHeaders } from "next/headers";

import { upstreamConfig } from "@/config/api";

export type UpstreamResult = {
  status: number;
  headers: Headers;
  body: ArrayBuffer;
};

export type UpstreamStreamResult = {
  status: number;
  headers: Headers;
  body: ReadableStream<Uint8Array> | null;
};

export const REQUEST_ID_HEADER = "X-Request-ID";

/** Same rule as the Go API (internal/platform/reqid): 1..128 of [A-Za-z0-9._:-]. */
const REQUEST_ID_PATTERN = /^[A-Za-z0-9._:-]{1,128}$/;

/**
 * Correlation id for one browser request: reuse a well-formed incoming
 * X-Request-ID, otherwise mint a UUID. Go echoes it in the X-Request-ID
 * response header, `meta.request_id`, its logs, and queued jobs.
 */
export function resolveRequestId(candidate?: string | null): string {
  if (candidate && REQUEST_ID_PATTERN.test(candidate)) return candidate;
  return crypto.randomUUID();
}

/** The Go API could not be reached (connection refused, DNS, reset, timeout). */
export class UpstreamUnavailableError extends Error {
  constructor(cause: unknown) {
    super("upstream unavailable", { cause });
    this.name = "UpstreamUnavailableError";
  }
}

/**
 * fetch() with a streamed request body (large uploads the BFF does not buffer)
 * must opt into half-duplex; buffered bodies need nothing extra.
 */
function upstreamBody(
  method: string,
  body: BodyInit | null | undefined,
): Pick<RequestInit, "body"> & { duplex?: "half" } {
  if (!body || method === "GET" || method === "HEAD")
    return { body: undefined };
  if (body instanceof ReadableStream) return { body, duplex: "half" };
  return { body };
}

async function fetchOrUnavailable(
  url: string,
  init: RequestInit & { duplex?: "half" },
): Promise<Response> {
  try {
    return await fetch(url, init);
  } catch (err) {
    // A client disconnect is not an API outage: let the caller see the abort.
    if (init.signal?.aborted) throw err;
    throw new UpstreamUnavailableError(err);
  }
}

function buildUpstreamUrl(pathWithQuery: string): string {
  return `${upstreamConfig.baseUrl.replace(/\/$/, "")}/${pathWithQuery.replace(/^\//, "")}`;
}

async function scrubUpstreamHeaders(init?: HeadersInit): Promise<Headers> {
  const headers = new Headers(init);
  headers.delete("host");
  headers.delete("connection");
  headers.delete("content-length");
  // Browser Origin/Cookie must not leak to Go — auth is Bearer only
  headers.delete("cookie");
  headers.delete("origin");
  // Callers never choose the client address; it comes from the incoming request only.
  headers.delete("x-forwarded-for");
  headers.delete("x-real-ip");
  // Callers (the BFF proxy) may pin one id for the whole browser request, retries included.
  headers.set(
    REQUEST_ID_HEADER,
    resolveRequestId(headers.get(REQUEST_ID_HEADER) ?? (await incomingRequestId())),
  );
  await forwardClientHeaders(headers);
  return headers;
}

async function incomingRequestId(): Promise<string | null> {
  try {
    return (await requestHeaders()).get(REQUEST_ID_HEADER);
  } catch {
    // Outside a request scope (build, background work).
    return null;
  }
}

/**
 * Pass the browser's address chain and user agent to Go, so IP rate limits and
 * session metadata see the end user instead of the BFF. The chain is forwarded
 * as received from the edge proxy (Traefik appends the real peer); Go only
 * honours it because the BFF is a trusted proxy (TRUSTED_PROXIES) and picks the
 * rightmost untrusted hop, so a client-supplied prefix cannot spoof it.
 */
async function forwardClientHeaders(headers: Headers): Promise<void> {
  let incoming: Headers;
  try {
    incoming = await requestHeaders();
  } catch {
    // Outside a request scope (build, background work): nothing to forward.
    return;
  }
  const forwardedFor = incoming.get("x-forwarded-for");
  if (forwardedFor) headers.set("X-Forwarded-For", forwardedFor);
  const userAgent = incoming.get("user-agent");
  if (userAgent && !headers.has("user-agent")) headers.set("User-Agent", userAgent);
}

/**
 * Forward a request to the Go API and buffer the body.
 * Caller attaches Authorization when needed.
 */
export async function fetchUpstream(
  pathWithQuery: string,
  init: {
    method: string;
    headers?: HeadersInit;
    body?: BodyInit | null;
  },
): Promise<UpstreamResult> {
  const response = await fetchOrUnavailable(buildUpstreamUrl(pathWithQuery), {
    method: init.method,
    headers: await scrubUpstreamHeaders(init.headers),
    ...upstreamBody(init.method, init.body),
    cache: "no-store",
    redirect: "manual",
  });

  return {
    status: response.status,
    headers: response.headers,
    body: await response.arrayBuffer(),
  };
}

/**
 * Forward a request and keep the body as a live stream (SSE / long-poll).
 * Do not call arrayBuffer() — that would buffer and break event streams.
 */
export async function fetchUpstreamStream(
  pathWithQuery: string,
  init: {
    method: string;
    headers?: HeadersInit;
    body?: BodyInit | null;
    signal?: AbortSignal;
  },
): Promise<UpstreamStreamResult> {
  const response = await fetchOrUnavailable(buildUpstreamUrl(pathWithQuery), {
    method: init.method,
    headers: await scrubUpstreamHeaders(init.headers),
    ...upstreamBody(init.method, init.body),
    cache: "no-store",
    signal: init.signal,
    redirect: "manual",
  });

  return {
    status: response.status,
    headers: response.headers,
    body: response.body,
  };
}
