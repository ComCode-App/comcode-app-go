// BFF proxy behaviour (TEC-1102): CSRF, path allowlist, shared refresh, response
// headers, upstream errors and request body streaming. bff-proxy.ts is transpiled
// with its auth-tokens / upstream imports swapped for in-memory stubs.
// Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { beforeEach, test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "bff-proxy-"));

const REWRITE = {
  "@/lib/server/auth-tokens": "./auth-tokens.mjs",
  "@/lib/server/upstream": "./upstream.mjs",
  "@/lib/server/bff-guard": "./bff-guard.mjs",
  "@/lib/server/refresh-single-flight": "./refresh-single-flight.mjs",
  "@/lib/observability/redact": "./redact.mjs",
};

function transpile(name, dir = here) {
  let { outputText } = ts.transpileModule(
    fs.readFileSync(path.join(dir, `${name}.ts`), "utf8"),
    {
      compilerOptions: {
        module: ts.ModuleKind.ESNext,
        target: ts.ScriptTarget.ES2022,
      },
    },
  );
  for (const [from, to] of Object.entries(REWRITE)) {
    outputText = outputText.replaceAll(`"${from}"`, `"${to}"`);
  }
  fs.writeFileSync(path.join(tmp, `${name}.mjs`), outputText);
}
transpile("bff-proxy");
transpile("bff-guard");
transpile("refresh-single-flight");
transpile("redact", path.join(here, "..", "observability"));

// --- stubs -----------------------------------------------------------------
fs.writeFileSync(
  path.join(tmp, "auth-tokens.mjs"),
  `const s = globalThis.__bff;
export async function getApiTokens() { return { ...s.tokens }; }
export async function persistApiTokens(input) {
  s.persisted.push(input);
  s.tokens = { ...s.tokens, accessToken: input.accessToken, refreshToken: input.refreshToken };
}
export async function clearAuthSessionCookie() { s.cleared += 1; }
`,
);
fs.writeFileSync(
  path.join(tmp, "upstream.mjs"),
  `const s = globalThis.__bff;
export const REQUEST_ID_HEADER = "X-Request-ID";
export function resolveRequestId(c) { return c || "rid-test"; }
export class UpstreamUnavailableError extends Error {
  constructor(cause) { super("upstream unavailable", { cause }); }
}
async function call(pathWithQuery, init) {
  s.calls.push({ path: pathWithQuery, init });
  return s.handler(pathWithQuery, init);
}
export async function fetchUpstream(p, init) {
  const r = await call(p, init);
  return { status: r.status, headers: new Headers(r.headers ?? {}), body: new TextEncoder().encode(r.body ?? "").buffer };
}
export async function fetchUpstreamStream(p, init) {
  const r = await call(p, init);
  const bytes = new TextEncoder().encode(r.body ?? "");
  return { status: r.status, headers: new Headers(r.headers ?? {}), body: new Blob([bytes]).stream() };
}
`,
);

const state = {};
globalThis.__bff = state;
const proxy = await import(pathToFileURL(path.join(tmp, "bff-proxy.mjs")).href);
const singleFlight = await import(
  pathToFileURL(path.join(tmp, "refresh-single-flight.mjs")).href
);

const ok = (data) => ({
  status: 200,
  headers: { "content-type": "application/json" },
  body: JSON.stringify({ success: true, data }),
});

beforeEach(() => {
  state.tokens = {
    accessToken: "old-access",
    refreshToken: "old-refresh",
    userId: "u1",
    email: null,
    impersonatorUuid: null,
  };
  state.persisted = [];
  state.cleared = 0;
  state.calls = [];
  state.handler = async () => ok({ ok: true });
  singleFlight.resetSharedRefresh();
  process.env.AUTH_URL = "https://app.comcode.test";
});

const APP = "https://app.comcode.test";

function browser(pathname, { method = "GET", headers = {}, body } = {}) {
  return new Request(`${APP}/api/v1/${pathname}`, {
    method,
    headers: { host: "app.comcode.test", ...headers },
    body,
  });
}

// --- CSRF ------------------------------------------------------------------
test("cross-site POST is blocked with the API's error envelope", async () => {
  const res = await proxy.handleProxyRequest(
    ["profiles"],
    browser("profiles", {
      method: "POST",
      headers: { origin: "https://evil.test", "sec-fetch-site": "cross-site" },
      body: "{}",
    }),
  );
  assert.equal(res.status, 403);
  const json = await res.json();
  assert.equal(json.success, false);
  assert.equal(json.error.code, "FORBIDDEN");
  assert.equal(state.calls.length, 0, "must not reach the API");
});

test("same-origin POST is forwarded", async () => {
  const res = await proxy.handleProxyRequest(
    ["profiles"],
    browser("profiles", {
      method: "POST",
      headers: {
        origin: APP,
        "sec-fetch-site": "same-origin",
        "content-type": "application/json",
      },
      body: "{}",
    }),
  );
  assert.equal(res.status, 200);
  assert.equal(state.calls.length, 1);
});

test("iyzico callback is exempt from the CSRF check", async () => {
  const res = await proxy.handleProxyRequest(
    ["public", "payment", "iyzico", "callback"],
    browser("public/payment/iyzico/callback", {
      method: "POST",
      headers: {
        origin: "https://sandbox-api.iyzipay.com",
        "sec-fetch-site": "cross-site",
        "content-type": "application/x-www-form-urlencoded",
      },
      body: "token=abc",
    }),
  );
  assert.equal(res.status, 200);
  assert.equal(state.calls.length, 1);
  assert.equal(
    state.calls[0].init.headers.get("Authorization"),
    null,
    "callbacks never carry the session",
  );
});

// --- path allowlist --------------------------------------------------------
test("path traversal and internal routes are rejected", async () => {
  for (const segs of [
    ["..", "healthz"],
    ["profiles", "..", "..", "x"],
    ["internal", "auth", "users"],
    ["a/b"],
    ["a\\b"],
  ]) {
    const res = await proxy.handleProxyRequest(segs, browser("x"));
    assert.equal(res.status, 404, JSON.stringify(segs));
    assert.equal((await res.json()).error.code, "NOT_FOUND");
  }
  assert.equal(state.calls.length, 0);
});

// --- parallel refresh ------------------------------------------------------
test("10 parallel requests with an expired token share one refresh and keep the cookie", async () => {
  let refreshes = 0;
  state.handler = async (p, init) => {
    if (p === "auth/refresh") {
      refreshes += 1;
      await new Promise((r) => setTimeout(r, 20));
      // Single-use refresh tokens: only the first rotation succeeds.
      if (refreshes > 1)
        return {
          status: 401,
          body: JSON.stringify({
            success: false,
            error: { code: "UNAUTHENTICATED" },
          }),
        };
      return ok({
        access_token: "new-access",
        refresh_token: "new-refresh",
        expires_in: 900,
      });
    }
    const auth = init.headers.get("Authorization");
    if (auth !== "Bearer new-access") return { status: 401, body: "{}" };
    return ok({ items: [] });
  };

  const responses = await Promise.all(
    Array.from({ length: 10 }, () =>
      proxy.handleProxyRequest(["profiles"], browser("profiles")),
    ),
  );
  assert.deepEqual(
    responses.map((r) => r.status),
    Array(10).fill(200),
  );
  assert.equal(refreshes, 1, "one upstream refresh");
  assert.equal(state.cleared, 0, "session cookie never cleared");
  assert.ok(state.persisted.every((p) => p.refreshToken === "new-refresh"));
});

test("a failed BFF refresh does not clear the session cookie", async () => {
  state.handler = async (p) =>
    p === "auth/refresh"
      ? { status: 401, body: "{}" }
      : { status: 401, body: "{}" };
  const res = await proxy.handleProxyRequest(["profiles"], browser("profiles"));
  assert.equal(res.status, 401);
  assert.equal(state.cleared, 0);
});

// --- response headers ------------------------------------------------------
test("Content-Length / Content-Encoding are not forwarded on a rewritten body", async () => {
  const upstreamBody = JSON.stringify({
    success: true,
    data: {
      access_token: "a".repeat(200),
      refresh_token: "r".repeat(200),
      expires_in: 900,
    },
  });
  state.handler = async () => ({
    status: 200,
    headers: {
      "content-type": "application/json",
      "content-length": String(upstreamBody.length),
      "content-encoding": "gzip",
    },
    body: upstreamBody,
  });
  const res = await proxy.handleProxyRequest(
    ["auth", "refresh"],
    browser("auth/refresh", {
      method: "POST",
      headers: { origin: APP, "content-type": "application/json" },
      body: "{}",
    }),
  );
  assert.equal(res.status, 200);
  assert.equal(res.headers.get("content-length"), null);
  assert.equal(res.headers.get("content-encoding"), null);
  const json = await res.json();
  assert.equal(json.data.access_token, undefined, "tokens stripped");
  assert.equal(json.data.authenticated, true);
});

test("buffered stream fallback and SSE drop Content-Length; binary passthrough keeps it", async () => {
  state.handler = async () => ({
    status: 200,
    headers: { "content-type": "application/json", "content-length": "999" },
    body: "{}",
  });
  const sse = await proxy.handleProxyRequest(
    ["notifications", "stream"],
    browser("notifications/stream", {
      headers: { accept: "text/event-stream" },
    }),
  );
  assert.equal(sse.headers.get("content-length"), null);

  state.handler = async () => ({
    status: 200,
    headers: { "content-type": "image/png", "content-length": "3" },
    body: "abc",
  });
  const bin = await proxy.handleProxyRequest(
    ["public", "storage", "x.png"],
    browser("public/storage/x.png"),
  );
  assert.equal(bin.headers.get("content-length"), "3");

  state.handler = async () => ({
    status: 200,
    headers: {
      "content-type": "image/png",
      "content-length": "3",
      "content-encoding": "gzip",
    },
    body: "abc",
  });
  const enc = await proxy.handleProxyRequest(
    ["public", "storage", "x.png"],
    browser("public/storage/x.png"),
  );
  assert.equal(enc.headers.get("content-length"), null);
});

// --- upstream errors -------------------------------------------------------
test("an unreachable API becomes a 502 envelope, other errors a 500 envelope", async () => {
  const { UpstreamUnavailableError } = await import(
    pathToFileURL(path.join(tmp, "upstream.mjs")).href
  );
  state.handler = async () => {
    throw new UpstreamUnavailableError(new Error("ECONNREFUSED"));
  };
  const res = await proxy.handleProxyRequest(["profiles"], browser("profiles"));
  assert.equal(res.status, 502);
  assert.equal((await res.json()).error.code, "UPSTREAM_UNAVAILABLE");

  state.handler = async () => {
    throw new Error("boom");
  };
  const res2 = await proxy.handleProxyRequest(
    ["profiles"],
    browser("profiles"),
  );
  assert.equal(res2.status, 500);
  assert.equal((await res2.json()).error.code, "INTERNAL_ERROR");
});

test("a request the browser cancelled is answered 499, not a 500 envelope", async () => {
  const controller = new AbortController();
  state.handler = async () => {
    controller.abort();
    throw new DOMException("The operation was aborted.", "AbortError");
  };
  const req = new Request(`${APP}/api/v1/profiles`, {
    headers: { host: "app.comcode.test" },
    signal: controller.signal,
  });
  const logged = { error: 0, info: 0 };
  const orig = { error: console.error, info: console.info };
  console.error = () => (logged.error += 1);
  console.info = () => (logged.info += 1);
  try {
    const res = await proxy.handleProxyRequest(["profiles"], req);
    assert.equal(res.status, 499);
  } finally {
    Object.assign(console, orig);
  }
  assert.equal(logged.error, 0, "a client cancel is not an error");
  assert.equal(logged.info, 1);
});

test("logged proxy errors are redacted", async () => {
  state.handler = async () => {
    throw new Error("failed for user ali@example.com with Bearer abc.def");
  };
  const lines = [];
  const orig = console.error;
  console.error = (...a) => lines.push(a.join(" "));
  try {
    const res = await proxy.handleProxyRequest(
      ["profiles"],
      browser("profiles"),
    );
    assert.equal(res.status, 500);
  } finally {
    console.error = orig;
  }
  assert.equal(lines.length, 1);
  assert.ok(!lines[0].includes("ali@example.com"), lines[0]);
  assert.ok(!lines[0].includes("abc.def"), lines[0]);
});

// --- request bodies --------------------------------------------------------
test("small bodies are buffered; large or unknown-length bodies are streamed", async () => {
  await proxy.handleProxyRequest(
    ["profiles"],
    browser("profiles", {
      method: "POST",
      headers: { origin: APP, "content-length": "2" },
      body: "{}",
    }),
  );
  assert.ok(state.calls[0].init.body instanceof ArrayBuffer);

  state.calls = [];
  await proxy.handleProxyRequest(
    ["platform", "storage", "objects"],
    browser("platform/storage/objects", {
      method: "POST",
      headers: {
        origin: APP,
        "content-type": "multipart/form-data; boundary=x",
        "content-length": String(proxy.BUFFERED_BODY_LIMIT + 1),
      },
      body: "--x--",
    }),
  );
  assert.ok(state.calls[0].init.body instanceof ReadableStream);
});

test("a declared body over the streaming cap is refused with 413", async () => {
  const res = await proxy.handleProxyRequest(
    ["platform", "storage", "objects"],
    browser("platform/storage/objects", {
      method: "POST",
      headers: {
        origin: APP,
        "content-length": String(proxy.STREAMED_BODY_LIMIT + 1),
      },
      body: "x",
    }),
  );
  assert.equal(res.status, 413);
  assert.equal(state.calls.length, 0);
});
