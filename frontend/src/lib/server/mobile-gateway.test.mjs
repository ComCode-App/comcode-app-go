// Mobile API gateway (TEC-1178): Bearer only, no cookies, tokens stay in the
// JSON, refused paths, forwarded client headers, body limits. mobile-gateway.ts
// is transpiled with its upstream import swapped for an in-memory stub.
// Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { beforeEach, test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "mobile-gateway-"));

const REWRITE = {
  "@/lib/server/upstream": "./upstream.mjs",
  "@/lib/server/bff-guard": "./bff-guard.mjs",
  "@/lib/server/proxy-body": "./proxy-body.mjs",
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
transpile("mobile-gateway");
transpile("bff-guard");
transpile("proxy-body");
transpile("redact", path.join(here, "..", "observability"));

fs.writeFileSync(
  path.join(tmp, "upstream.mjs"),
  `const s = globalThis.__mobile;
export const REQUEST_ID_HEADER = "X-Request-ID";
export function resolveRequestId(c) { return c || "rid-test"; }
export class UpstreamUnavailableError extends Error {
  constructor(cause) { super("upstream unavailable", { cause }); }
}
export async function fetchUpstream() { throw new Error("gateway must stream"); }
export async function fetchUpstreamStream(p, init) {
  s.calls.push({ path: p, init });
  const r = await s.handler(p, init);
  const bytes = new TextEncoder().encode(r.body ?? "");
  return { status: r.status, headers: new Headers(r.headers ?? {}), body: new Blob([bytes]).stream() };
}
`,
);

const state = {};
globalThis.__mobile = state;
const gateway = await import(
  pathToFileURL(path.join(tmp, "mobile-gateway.mjs")).href
);
const bodyLimits = await import(
  pathToFileURL(path.join(tmp, "proxy-body.mjs")).href
);

const tokens = {
  access_token: "acc.ess.jwt",
  refresh_token: "opaque-refresh",
  expires_in: 900,
  token_type: "Bearer",
};

beforeEach(() => {
  state.calls = [];
  state.handler = async () => ({
    status: 200,
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ success: true, data: { ok: true } }),
  });
});

const SITE = "https://app.comcode.test";
const APP_HEADERS = {
  "x-client": "comcode-mobile",
  "x-app-version": "1.0.3",
  "x-platform": "ios",
};

function mobile(pathname, { method = "GET", headers = {}, body } = {}) {
  return new Request(`${SITE}/api/mobile/v1/${pathname}`, {
    method,
    headers: { host: "app.comcode.test", ...APP_HEADERS, ...headers },
    body,
  });
}

const segs = (p) => p.split("/");

test("the gateway source never touches cookies or the Auth.js session", () => {
  const src = fs.readFileSync(path.join(here, "mobile-gateway.ts"), "utf8");
  const imports = [...src.matchAll(/from "([^"]+)"/g)].map((m) => m[1]);
  assert.ok(imports.length > 0);
  for (const mod of imports) {
    assert.ok(!mod.includes("auth-tokens"), `must not import ${mod}`);
    assert.ok(!mod.startsWith("next/"), `must not import ${mod}`);
    assert.ok(!mod.includes("next-auth"), `must not import ${mod}`);
  }
  assert.ok(!/\bcookies\(\)/.test(src), "must not call cookies()");
});

test("a request with only the session cookie reaches Go without credentials", async () => {
  await gateway.handleMobileGatewayRequest(
    segs("auth/me"),
    mobile("auth/me", {
      headers: { cookie: "__Secure-authjs.session-token=abc; other=1" },
    }),
  );
  assert.equal(state.calls.length, 1);
  const sent = state.calls[0].init.headers;
  assert.equal(sent.get("authorization"), null);
  assert.equal(sent.get("cookie"), null);
});

test("the client's Bearer token is forwarded; malformed Authorization is dropped", async () => {
  await gateway.handleMobileGatewayRequest(
    segs("auth/me"),
    mobile("auth/me", { headers: { authorization: "Bearer acc.ess.jwt" } }),
  );
  assert.equal(
    state.calls[0].init.headers.get("authorization"),
    "Bearer acc.ess.jwt",
  );

  for (const bad of ["Basic dTpw", "Bearer a b", "Bearer", "acc.ess.jwt"]) {
    state.calls = [];
    await gateway.handleMobileGatewayRequest(
      segs("auth/me"),
      mobile("auth/me", { headers: { authorization: bad } }),
    );
    assert.equal(state.calls[0].init.headers.get("authorization"), null, bad);
  }
});

test("login, refresh and email-code responses keep the token pair in the JSON", async () => {
  state.handler = async () => ({
    status: 200,
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ success: true, data: tokens }),
  });
  for (const p of [
    "auth/login",
    "auth/refresh",
    "auth/email-code/verify",
    "auth/accounts/switch",
  ]) {
    const body =
      p === "auth/refresh"
        ? JSON.stringify({ refresh_token: "old-refresh" })
        : JSON.stringify({ email: "a@b.test", password: "x" });
    state.calls = [];
    const res = await gateway.handleMobileGatewayRequest(
      segs(p),
      mobile(p, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          "content-length": String(body.length),
        },
        body,
      }),
    );
    assert.equal(res.status, 200, p);
    const json = await res.json();
    assert.equal(json.data.access_token, tokens.access_token, p);
    assert.equal(json.data.refresh_token, tokens.refresh_token, p);
    // The app's own body is passed through untouched (refresh token included).
    assert.equal(
      new TextDecoder().decode(state.calls[0].init.body),
      body,
      `${p} body`,
    );
    assert.equal(state.calls[0].path, p);
  }
});

test("upstream Set-Cookie (and other non-allowlisted headers) never reach the app", async () => {
  state.handler = async () => ({
    status: 200,
    headers: {
      "content-type": "application/json",
      "set-cookie": "__Secure-authjs.session-token=x; Path=/; HttpOnly",
      "x-internal": "1",
      "x-request-id": "rid-up",
    },
    body: JSON.stringify({ success: true, data: tokens }),
  });
  const res = await gateway.handleMobileGatewayRequest(
    segs("auth/login"),
    mobile("auth/login", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: "{}",
    }),
  );
  assert.equal(res.headers.get("set-cookie"), null);
  assert.equal(res.headers.get("x-internal"), null);
  assert.equal(res.headers.get("x-request-id"), "rid-up");
  assert.equal(res.headers.get("access-control-allow-origin"), null);
});

test("internal, payment callback and QR exchange paths are refused with 404", async () => {
  for (const p of [
    "internal/auth/users",
    "internal/auth/session",
    "public/payment/iyzico/callback",
    "public/payment/paytr/callback",
    "auth/qr/exchange",
  ]) {
    const res = await gateway.handleMobileGatewayRequest(
      segs(p),
      mobile(p, { method: "POST", body: "{}" }),
    );
    assert.equal(res.status, 404, p);
    const json = await res.json();
    assert.equal(json.success, false);
    assert.equal(json.error.code, "NOT_FOUND");
  }
  for (const s of [[".."], ["profiles", "..", "internal"], ["a/b"]]) {
    const res = await gateway.handleMobileGatewayRequest(s, mobile("x"));
    assert.equal(res.status, 404, JSON.stringify(s));
  }
  assert.equal(state.calls.length, 0, "refused paths never reach Go");
});

test("app identity headers are forwarded; junk values are dropped", async () => {
  await gateway.handleMobileGatewayRequest(
    segs("auth/me"),
    mobile("auth/me", {
      headers: { "accept-language": "tr-TR,tr;q=0.9", "x-request-id": "r-1" },
    }),
  );
  let sent = state.calls[0].init.headers;
  assert.equal(sent.get("x-client"), "comcode-mobile");
  assert.equal(sent.get("x-app-version"), "1.0.3");
  assert.equal(sent.get("x-platform"), "ios");
  assert.equal(sent.get("accept-language"), "tr-TR,tr;q=0.9");
  assert.equal(sent.get("x-request-id"), "r-1");
  // X-Forwarded-For / User-Agent are added by upstream.ts from the incoming request.
  assert.equal(sent.get("x-forwarded-for"), null);

  state.calls = [];
  await gateway.handleMobileGatewayRequest(
    segs("auth/me"),
    mobile("auth/me", {
      headers: {
        "x-app-version": "1.0.3<script>",
        "x-platform": "x".repeat(65),
      },
    }),
  );
  sent = state.calls[0].init.headers;
  assert.equal(sent.get("x-app-version"), null);
  assert.equal(sent.get("x-platform"), null);
});

test("Go's 426 APP_UPDATE_REQUIRED is passed through unchanged", async () => {
  const envelope = {
    success: false,
    error: {
      code: "APP_UPDATE_REQUIRED",
      message: "app update required",
      details: [{ field: "min_version", message: "1.2.0" }],
    },
  };
  state.handler = async () => ({
    status: 426,
    headers: { "content-type": "application/json" },
    body: JSON.stringify(envelope),
  });
  const res = await gateway.handleMobileGatewayRequest(
    segs("auth/me"),
    mobile("auth/me"),
  );
  assert.equal(res.status, 426);
  assert.deepEqual(await res.json(), envelope);
});

test("body limits match the BFF: small bodies buffered, large streamed, oversized 413", async () => {
  await gateway.handleMobileGatewayRequest(
    segs("profiles"),
    mobile("profiles", {
      method: "POST",
      headers: { "content-length": "2" },
      body: "{}",
    }),
  );
  assert.ok(state.calls[0].init.body instanceof ArrayBuffer);

  state.calls = [];
  await gateway.handleMobileGatewayRequest(
    segs("profiles/x/media"),
    mobile("profiles/x/media", {
      method: "POST",
      headers: {
        "content-type": "multipart/form-data; boundary=x",
        "content-length": String(bodyLimits.BUFFERED_BODY_LIMIT + 1),
      },
      body: "--x--",
    }),
  );
  assert.ok(state.calls[0].init.body instanceof ReadableStream);

  state.calls = [];
  const res = await gateway.handleMobileGatewayRequest(
    segs("profiles/x/media"),
    mobile("profiles/x/media", {
      method: "POST",
      headers: {
        "content-length": String(bodyLimits.STREAMED_BODY_LIMIT + 1),
      },
      body: "x",
    }),
  );
  assert.equal(res.status, 413);
  assert.equal((await res.json()).error.code, "PAYLOAD_TOO_LARGE");
  assert.equal(state.calls.length, 0);
});

test("an unreachable API becomes a 502 envelope", async () => {
  const { UpstreamUnavailableError } = await import(
    pathToFileURL(path.join(tmp, "upstream.mjs")).href
  );
  state.handler = async () => {
    throw new UpstreamUnavailableError(new Error("ECONNREFUSED"));
  };
  const orig = console.error;
  console.error = () => {};
  try {
    const res = await gateway.handleMobileGatewayRequest(
      segs("auth/me"),
      mobile("auth/me"),
    );
    assert.equal(res.status, 502);
    assert.equal((await res.json()).error.code, "UPSTREAM_UNAVAILABLE");
  } finally {
    console.error = orig;
  }
});
