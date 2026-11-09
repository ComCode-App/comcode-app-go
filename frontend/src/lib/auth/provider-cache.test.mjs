// NextAuth provider cache (TEC-1113). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const { outputText } = ts.transpileModule(
  fs.readFileSync(path.join(here, "provider-cache.ts"), "utf8"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
);
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "provider-cache-"));
const compiled = path.join(tmp, "provider-cache.mjs");
fs.writeFileSync(compiled, outputText);
const { createProviderCache, PROVIDERS_CACHE_TTL_MS } = await import(
  pathToFileURL(compiled).href
);

test("TTL is a single 60 second constant", () => {
  assert.equal(PROVIDERS_CACHE_TTL_MS, 60_000);
});

test("serves the cached build until the TTL expires", async () => {
  let clock = 0;
  let builds = 0;
  const cache = createProviderCache(
    async () => ({ value: ++builds, complete: true }),
    1000,
    () => clock,
  );
  assert.equal(await cache.get(), 1);
  clock = 999;
  assert.equal(await cache.get(), 1);
  clock = 1000;
  assert.equal(await cache.get(), 2);
  assert.equal(builds, 2);
});

test("concurrent cold requests share one build", async () => {
  let builds = 0;
  let release;
  const gate = new Promise((r) => (release = r));
  const cache = createProviderCache(async () => {
    builds += 1;
    await gate;
    return { value: "providers", complete: true };
  });
  const all = Promise.all([cache.get(), cache.get(), cache.get()]);
  release();
  assert.deepEqual(await all, ["providers", "providers", "providers"]);
  assert.equal(builds, 1);
});

test("an incomplete build is returned but not cached", async () => {
  let builds = 0;
  const cache = createProviderCache(async () => {
    builds += 1;
    return { value: builds, complete: builds > 1 };
  });
  assert.equal(await cache.get(), 1);
  assert.equal(await cache.get(), 2);
  assert.equal(await cache.get(), 2);
  assert.equal(builds, 2);
});

test("a failed build is not cached and the next call retries", async () => {
  let builds = 0;
  const cache = createProviderCache(async () => {
    builds += 1;
    if (builds === 1) throw new Error("api down");
    return { value: "ok", complete: true };
  });
  await assert.rejects(cache.get(), /api down/);
  assert.equal(await cache.get(), "ok");
});

test("invalidate forces a rebuild", async () => {
  let builds = 0;
  const cache = createProviderCache(async () => ({
    value: ++builds,
    complete: true,
  }));
  assert.equal(await cache.get(), 1);
  cache.invalidate();
  assert.equal(await cache.get(), 2);
});
