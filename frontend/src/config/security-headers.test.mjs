// Security headers from next.config.ts (TEC-1102). Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
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

/** Minimal matcher for the `/x`, `/x/:path*` and `/:path*` sources used here. */
function headersFor(rules, pathname) {
  const out = new Map();
  for (const rule of rules) {
    const src = rule.source;
    let hit;
    if (src === "/:path*") hit = true;
    else if (src.endsWith("/:path*"))
      hit = pathname.startsWith(src.slice(0, -"/:path*".length) + "/");
    else hit = pathname === src;
    if (hit) for (const h of rule.headers) out.set(h.key, h.value);
  }
  return out;
}

test("every (shell) route folder gets framing protection", () => {
  const shell = path.join(here, "..", "app", "(shell)");
  const routes = [];
  for (const entry of fs.readdirSync(shell, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue;
    if (entry.name.startsWith("(")) {
      for (const sub of fs.readdirSync(path.join(shell, entry.name), {
        withFileTypes: true,
      })) {
        if (sub.isDirectory()) routes.push(sub.name);
      }
    } else {
      routes.push(entry.name);
    }
  }
  assert.ok(routes.length > 5);
  for (const r of routes) {
    assert.ok(
      mod.APP_ROUTE_PREFIXES.includes(r),
      `add "${r}" to APP_ROUTE_PREFIXES`,
    );
  }
});

test("panel routes deny framing; public profiles stay frameable (TEC-1102 decision pending)", () => {
  const rules = mod.securityHeaderRules(true);
  for (const p of [
    "/",
    "/billing",
    "/company/members",
    "/platform/users",
    "/api/v1/profiles",
    "/login",
  ]) {
    const h = headersFor(rules, p);
    assert.equal(h.get("X-Frame-Options"), "SAMEORIGIN", p);
    assert.match(
      h.get("Content-Security-Policy") ?? "",
      /frame-ancestors 'self'/,
      p,
    );
  }
  for (const p of ["/someuser", "/c/abc123", "/s/my-link"]) {
    const h = headersFor(rules, p);
    assert.equal(h.get("X-Frame-Options"), undefined, p);
    assert.equal(h.get("X-Content-Type-Options"), "nosniff", p);
  }
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
