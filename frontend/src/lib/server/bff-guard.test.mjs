// BFF path allowlist and CSRF origin check (TEC-1102). Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const { outputText } = ts.transpileModule(
  fs.readFileSync(path.join(here, "bff-guard.ts"), "utf8"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
);
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "bff-guard-"));
fs.writeFileSync(path.join(tmp, "bff-guard.mjs"), outputText);
const guard = await import(pathToFileURL(path.join(tmp, "bff-guard.mjs")).href);

const ORIGINS = new Set(["https://app.comcode.test"]);

function req(method, headers = {}) {
  return new Request("http://127.0.0.1:54002/api/v1/profiles", {
    method,
    headers,
  });
}

test("path allowlist refuses traversal, encoded separators and internal routes", () => {
  const bad = [
    [],
    [".."],
    ["profiles", "..", "internal", "auth"],
    ["profiles", "."],
    ["profiles", ""],
    ["a/b"], // %2F decoded by Next
    ["..%2F"],
    ["%2e%2e"], // double-encoded
    ["a\\b"],
    ["a?b"],
    ["a#b"],
    ["a\u0000b"],
    ["internal", "auth", "users"],
    ["INTERNAL", "auth"],
  ];
  for (const segs of bad) {
    assert.equal(guard.isForbiddenProxyPath(segs), true, JSON.stringify(segs));
  }
  for (const segs of [
    ["profiles"],
    ["auth", "refresh"],
    ["public", "payment", "iyzico", "callback"],
    ["profiles", "a.b..c"],
  ]) {
    assert.equal(guard.isForbiddenProxyPath(segs), false, JSON.stringify(segs));
  }
});

test("cross-site mutations are blocked, same-origin and safe methods pass", () => {
  assert.equal(
    guard.isCrossSiteMutation(
      req("POST", { "sec-fetch-site": "cross-site" }),
      ORIGINS,
    ),
    true,
  );
  assert.equal(
    guard.isCrossSiteMutation(
      req("DELETE", { origin: "https://evil.test" }),
      ORIGINS,
    ),
    true,
  );
  assert.equal(
    guard.isCrossSiteMutation(req("PATCH", { origin: "null" }), ORIGINS),
    true,
  );
  assert.equal(
    guard.isCrossSiteMutation(
      req("PUT", {
        "sec-fetch-site": "same-site",
        origin: "https://api.comcode.test",
      }),
      ORIGINS,
    ),
    true,
  );
  assert.equal(
    guard.isCrossSiteMutation(
      req("POST", {
        "sec-fetch-site": "same-origin",
        origin: "https://app.comcode.test",
      }),
      ORIGINS,
    ),
    false,
  );
  // Non-browser client: no Origin, no Sec-Fetch-Site.
  assert.equal(guard.isCrossSiteMutation(req("POST"), ORIGINS), false);
  assert.equal(
    guard.isCrossSiteMutation(
      req("GET", { "sec-fetch-site": "cross-site" }),
      ORIGINS,
    ),
    false,
  );
});

test("allowed origins come from AUTH_URL / PUBLIC_FRONTEND_URL and the forwarded host", () => {
  const behindTraefik = new Headers({
    host: "frontend:54002",
    "x-forwarded-host": "app.comcode.test",
    "x-forwarded-proto": "https",
  });
  const origins = guard.allowedAppOrigins(behindTraefik, {
    AUTH_URL: "https://comcode.test/",
    PUBLIC_FRONTEND_URL: "https://www.comcode.test",
  });
  assert.ok(origins.has("https://comcode.test"));
  assert.ok(origins.has("https://www.comcode.test"));
  assert.ok(origins.has("https://app.comcode.test"));
  assert.ok(!origins.has("http://app.comcode.test"));

  const local = guard.allowedAppOrigins(
    new Headers({ host: "localhost:54002" }),
    {},
  );
  assert.ok(local.has("http://localhost:54002"));
});

test("payment callbacks are the only CSRF exemptions", () => {
  assert.equal(guard.isCsrfExemptPath("public/payment/iyzico/callback"), true);
  assert.equal(guard.isCsrfExemptPath("public/payment/paytr/callback"), true);
  assert.equal(guard.isCsrfExemptPath("public/leads"), false);
  assert.equal(guard.isCsrfExemptPath("webhooks"), false);
});

test("jwtExpiresAt reads exp without verifying", () => {
  const payload = Buffer.from(JSON.stringify({ exp: 1700000000 })).toString(
    "base64url",
  );
  assert.equal(guard.jwtExpiresAt(`h.${payload}.s`), 1700000000);
  assert.equal(guard.jwtExpiresAt("garbage"), null);
  assert.equal(guard.jwtExpiresAt(null), null);
});
