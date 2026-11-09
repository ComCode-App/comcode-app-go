// Security headers from next.config.ts (TEC-1102). Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const { outputText } = ts.transpileModule(
  fs.readFileSync(path.join(here, "security-headers.ts"), "utf8"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
);
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "security-headers-"));
fs.writeFileSync(path.join(tmp, "security-headers.mjs"), outputText);
const mod = await import(
  pathToFileURL(path.join(tmp, "security-headers.mjs")).href
);

// Next's own runtime matcher for `headers()` sources
// (next/dist/server/lib/router-utils/filesystem.js), so regex sources such as
// the payment-result negative lookahead are tested as Next evaluates them.
const require = createRequire(import.meta.url);
const {
  getPathMatch,
} = require("next/dist/shared/lib/router/utils/path-match");
const { modifyRouteRegex } = require("next/dist/lib/redirect-status");

/** Headers Next sends for `pathname`: every matching rule, later keys win. */
function headersFor(rules, pathname) {
  const out = new Map();
  for (const rule of rules) {
    const match = getPathMatch(rule.source, {
      strict: true,
      removeUnnamedParams: true,
      regexModifier: (regex) => modifyRouteRegex(regex),
    });
    if (match(pathname)) for (const h of rule.headers) out.set(h.key, h.value);
  }
  return out;
}

/** Every top-level route folder under app/, route groups flattened. */
function topLevelRoutes() {
  const out = [];
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      if (!entry.isDirectory()) continue;
      if (entry.name.startsWith("(")) walk(path.join(dir, entry.name));
      else out.push(entry.name);
    }
  };
  walk(path.join(here, "..", "app"));
  return out;
}

function assertDeny(h, p) {
  assert.equal(h.get("X-Frame-Options"), "DENY", p);
  assert.match(
    h.get("Content-Security-Policy") ?? "",
    /frame-ancestors 'none'/,
    p,
  );
}

test("public visitor pages cannot be framed (TEC-1119)", () => {
  const rules = mod.securityHeaderRules(true);
  for (const p of [
    "/someuser",
    "/c/abc123",
    "/s/my-link",
    "/share/some-slug",
    "/share/s/some-token",
    "/kvkk",
  ]) {
    const h = headersFor(rules, p);
    assertDeny(h, p);
    assert.equal(h.get("X-Content-Type-Options"), "nosniff", p);
  }
});

test("panel routes and every app route folder deny framing", () => {
  const rules = mod.securityHeaderRules(true);
  const routes = topLevelRoutes().filter((r) => r !== "api");
  assert.ok(routes.length > 10);
  const paths = [
    "/",
    "/billing",
    "/company/members",
    "/platform/users",
    "/login",
    "/api/v1/profiles",
    "/api/auth/session",
    ...routes.map((r) => `/${r.replace(/^\[.*\]$/, "someone")}`),
  ];
  for (const p of paths) assertDeny(headersFor(rules, p), p);
});

test("only the storage previews the app frames itself allow same-origin framing", () => {
  const rules = mod.securityHeaderRules(true);
  for (const p of [
    "/api/v1/platform/storage/objects/preview",
    "/api/v1/public/storage/some-slug",
    "/api/v1/public/storage/s/some-token",
  ]) {
    const h = headersFor(rules, p);
    assert.equal(h.get("X-Frame-Options"), "SAMEORIGIN", p);
    assert.equal(h.get("Content-Security-Policy"), "frame-ancestors 'self'", p);
    assert.equal(h.get("X-Content-Type-Options"), "nosniff", p);
  }
});

const SESSION = "0b9f3c3e-6c3a-4a5e-9d63-1f1d2c3b4a5e";
const PAYMENT_RESULT_PATHS = [
  `/billing/checkout/${SESSION}`,
  `/billing/cards/add/${SESSION}`,
  `/orders/checkout/${SESSION}`,
];

test("PayTR may frame the payment result pages, without X-Frame-Options (TEC-1125)", () => {
  const rules = mod.securityHeaderRules(true);
  for (const p of PAYMENT_RESULT_PATHS) {
    const h = headersFor(rules, p);
    assert.equal(
      h.get("Content-Security-Policy"),
      "frame-ancestors 'self' https://www.paytr.com",
      p,
    );
    assert.equal(h.get("X-Frame-Options"), undefined, p);
    assert.equal(h.get("X-Content-Type-Options"), "nosniff", p);
    assert.equal(mod.isPaymentResultPath(p), true, p);
  }
});

test("other billing / orders pages still deny framing (TEC-1125)", () => {
  const rules = mod.securityHeaderRules(true);
  for (const p of [
    "/billing",
    "/billing/success",
    "/billing/checkout",
    "/billing/cards",
    "/billing/cards/add",
    `/billing/checkout/${SESSION}/extra`,
    `/billing/cards/add/${SESSION}/x`,
    "/orders",
    "/orders/checkout",
    `/orders/${SESSION}`,
    `/orders/checkout/${SESSION}/extra`,
    `/x/billing/checkout/${SESSION}`,
    `/someuser/orders/checkout`,
  ]) {
    assertDeny(headersFor(rules, p), p);
    assert.equal(mod.isPaymentResultPath(p), false, p);
  }
});

test("payment result prefixes match the URLs the backend builds", () => {
  const usecase = path.join(
    here,
    "../../../backend/internal/modules/billing/usecase",
  );
  const built = new Set();
  for (const file of fs.readdirSync(usecase)) {
    if (!file.endsWith(".go") || file.endsWith("_test.go")) continue;
    const src = fs.readFileSync(path.join(usecase, file), "utf8");
    for (const m of src.matchAll(/"%s(\/[a-z/]+)\/%s", s\.frontendURL/g))
      built.add(m[1]);
  }
  assert.deepEqual([...built].sort(), [...mod.PAYMENT_RESULT_PREFIXES].sort());
});

test("baseline headers; HSTS only in production; no script-src", () => {
  const prod = headersFor(mod.securityHeaderRules(true), "/billing");
  assert.equal(prod.get("X-Content-Type-Options"), "nosniff");
  assert.equal(prod.get("Referrer-Policy"), "strict-origin-when-cross-origin");
  assert.match(
    prod.get("Permissions-Policy"),
    /camera=\(\).*microphone=\(\).*geolocation=\(\)/,
  );
  assert.match(prod.get("Strict-Transport-Security"), /max-age=31536000/);
  assert.doesNotMatch(prod.get("Content-Security-Policy"), /script-src/);
  const dev = headersFor(mod.securityHeaderRules(false), "/billing");
  assert.equal(dev.get("Strict-Transport-Security"), undefined);
});
