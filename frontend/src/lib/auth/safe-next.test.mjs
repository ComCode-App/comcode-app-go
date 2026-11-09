// Open-redirect guard for ?next= / callbackUrl (TEC-1103). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, "safe-next.ts"), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "safe-next-"));
const compiled = path.join(tmp, "safe-next.mjs");
fs.writeFileSync(compiled, outputText);

const { safeNextPath, loginWithNext } = await import(
  pathToFileURL(compiled).href
);

const FALLBACK = "/home";

test("rejects off-site and script targets", () => {
  const unsafe = [
    "//evil.com",
    "/\\evil.com",
    "\\\\evil.com",
    "\\/evil.com",
    "https://evil.com",
    "http:/evil.com",
    "/%2f%2fevil.com",
    "/%2F%2Fevil.com",
    "/%5cevil.com",
    "/%5Cevil.com",
    "/%09/evil.com",
    "javascript:alert(1)",
    "JavaScript:alert(1)",
    " /x",
    "\t/x",
    "\n/x",
    "/\t/evil.com",
    "/\n/evil.com",
    "/x\u0000",
    "evil.com",
    "",
    null,
    undefined,
  ];
  for (const raw of unsafe) {
    assert.equal(safeNextPath(raw, FALLBACK), FALLBACK, JSON.stringify(raw));
  }
});

test("rejects sign-in loops and API paths", () => {
  for (const raw of [
    "/login",
    "/platform/login?next=/x",
    "/platform/register/",
    "/PLATFORM/LOGIN",
    "/api/v1/me",
    "/api",
    "/%61pi/v1/me",
  ]) {
    assert.equal(safeNextPath(raw, FALLBACK), FALLBACK, raw);
  }
});

test("accepts same-origin paths and keeps query and hash", () => {
  for (const raw of [
    "/",
    "/dashboard?x=1#a",
    "/platform/invite/abc123",
    "/profiles/42/edit?tab=links",
    "/search?q=100%",
    "/search?q=a%20b",
    "/apidocs",
  ]) {
    assert.equal(safeNextPath(raw, FALLBACK), raw, raw);
  }
});

test("null fallback is returned as-is", () => {
  assert.equal(safeNextPath("//evil.com", null), null);
});

test("loginWithNext keeps safe deep links and drops unsafe ones", () => {
  assert.equal(
    loginWithNext("/platform/login", "/orders?open=1"),
    "/platform/login?next=%2Forders%3Fopen%3D1",
  );
  assert.equal(
    loginWithNext("/platform/login", "//evil.com"),
    "/platform/login",
  );
  assert.equal(loginWithNext("/platform/login", null), "/platform/login");
});
