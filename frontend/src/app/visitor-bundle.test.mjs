// Visitor pages (/{username}, /c/*, /s/*, /kvkk/*) sit outside app/(shell) and only get the root
// PublicProviders (TEC-858). This walks their static import graph and fails if it reaches a
// panel-only provider or library, or a t() namespace that is not loaded on visitor pages.
// Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const src = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

const ENTRIES = ["app/layout.tsx", "app/[username]", "app/c", "app/s", "app/kvkk"];

const FORBIDDEN = [
  "providers/app-providers",
  "providers/auth-provider",
  "providers/permission-provider",
  "providers/dialog-provider",
  "providers/query-provider",
  "providers/realtime-provider",
  "lib/i18n/app-catalogs",
  "features/step-up-engine/index",
  "@tanstack/react-query",
  "next-auth",
  "centrifuge",
  // Error tracking SDK: visitor pages only load it through the lazy catcher's dynamic
  // import("./sdk") after the first error (TEC-952); a static import would ship it always.
  "@sentry/browser",
  "lib/error-tracking/sdk",
];

function resolveLocal(spec, fromFile) {
  let base;
  if (spec.startsWith("@/")) base = path.join(src, spec.slice(2));
  else if (spec.startsWith(".")) base = path.resolve(path.dirname(fromFile), spec);
  else return null;
  for (const candidate of [base, ...[".ts", ".tsx", ".js", ".mjs"].map((e) => base + e), path.join(base, "index.ts"), path.join(base, "index.tsx")]) {
    if (fs.existsSync(candidate) && fs.statSync(candidate).isFile()) return candidate;
  }
  return null;
}

function entryFiles() {
  const out = [];
  for (const entry of ENTRIES) {
    const abs = path.join(src, entry);
    if (fs.statSync(abs).isFile()) out.push(abs);
    else {
      for (const f of fs.readdirSync(abs, { recursive: true })) {
        if (/\.(ts|tsx)$/.test(f) && !/\.test\./.test(f)) out.push(path.join(abs, f));
      }
    }
  }
  return out;
}

function walk() {
  const seen = new Set();
  const packages = new Set();
  const stack = entryFiles();
  while (stack.length) {
    const file = stack.pop();
    if (seen.has(file)) continue;
    seen.add(file);
    if (!/\.(ts|tsx|js|mjs)$/.test(file)) continue;
    const code = fs.readFileSync(file, "utf8");
    // `import type` is erased at build time and never ships.
    for (const m of code.matchAll(/^\s*(?:import|export)\s+(?!type\s)(?:[^"';]*?\sfrom\s+)?["']([^"']+)["']/gm)) {
      const spec = m[1];
      const local = resolveLocal(spec, file);
      if (local) stack.push(local);
      else if (!spec.startsWith(".") && !spec.startsWith("@/")) packages.add(spec);
    }
  }
  return { files: [...seen], packages: [...packages] };
}

const { files, packages } = walk();
const rel = (f) => path.relative(src, f).split(path.sep).join("/");

test("visitor pages do not import panel-only providers or libraries", () => {
  const hits = [
    ...files.map(rel).filter((f) => FORBIDDEN.some((x) => f.replace(/\.(ts|tsx)$/, "") === x)),
    ...packages.filter((p) => FORBIDDEN.some((x) => p === x || p.startsWith(`${x}/`))),
  ];
  assert.deepEqual(hits, [], `visitor import graph reaches: ${hits.join(", ")}`);
});

test("visitor pages only use i18n namespaces loaded on visitor pages", () => {
  const messages = fs.readFileSync(path.join(src, "lib/i18n/messages.ts"), "utf8");
  const block = messages.match(/tr:\s*\{([\s\S]*?)\},\s*\n\s*en:\s*\{/)[1];
  const publicNs = new Set([...block.matchAll(/^\s*([a-z][a-z0-9_]*)\s*:/gm)].map((m) => m[1]));
  const missing = new Set();
  for (const file of files) {
    if (!/\.(ts|tsx)$/.test(file)) continue;
    const code = fs.readFileSync(file, "utf8");
    for (const m of code.matchAll(/\b(?:t|tp)\(\s*["'`]([a-z_]+)\./g)) {
      if (!publicNs.has(m[1])) missing.add(`${m[1]} (${rel(file)})`);
    }
  }
  assert.deepEqual([...missing], [], `namespaces not in lib/i18n/messages.ts: ${[...missing].join(", ")}`);
});
