// Apple form_post callback cookies and the Auth.js cross-site POST guard (TEC-1123).
// Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "apple-form-post-"));
for (const [name, file] of [
  ["apple-form-post", path.join(here, "apple-form-post.ts")],
  ["bff-guard", path.join(here, "../server/bff-guard.ts")],
]) {
  const { outputText } = ts.transpileModule(fs.readFileSync(file, "utf8"), {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  });
  fs.writeFileSync(path.join(tmp, `${name}.mjs`), outputText);
}
const apple = await import(
  pathToFileURL(path.join(tmp, "apple-form-post.mjs")).href
);
const guard = await import(pathToFileURL(path.join(tmp, "bff-guard.mjs")).href);

test("only the Apple sign-in and callback paths are the Apple flow", () => {
  for (const p of [
    "/api/auth/signin/apple",
    "/api/auth/callback/apple",
    "/api/auth/callback/apple/",
  ]) {
    assert.equal(apple.isAppleFlowPath(p), true, p);
  }
  for (const p of [
    "/api/auth/callback/google",
    "/api/auth/signin/github",
    "/api/auth/session",
    "/api/auth/csrf",
    "/api/auth/callback/apple/extra",
    "/api/auth/callback/applex",
    "/api/v1/auth/callback/apple",
    "/x/api/auth/callback/apple",
    "",
  ]) {
    assert.equal(apple.isAppleFlowPath(p), false, p);
  }
});

test("Apple flow over https: check cookies are SameSite=None; Secure", () => {
  const cookies = apple.appleFormPostCookies("/api/auth/callback/apple", true);
  assert.ok(cookies);
  assert.deepEqual(Object.keys(cookies).sort(), [
    "callbackUrl",
    "nonce",
    "pkceCodeVerifier",
    "state",
  ]);
  for (const [key, c] of Object.entries(cookies)) {
    assert.equal(c.options.sameSite, "none", key);
    assert.equal(c.options.secure, true, key);
    assert.equal(c.options.httpOnly, true, key);
    assert.equal(c.options.path, "/", key);
    assert.ok(c.name.startsWith("__Secure-"), key);
    assert.ok(c.options.maxAge > 0 && c.options.maxAge <= 900, key);
  }
  // Same names as the Auth.js secure defaults, so sign-in and callback agree.
  assert.equal(cookies.state.name, "__Secure-authjs.state");
  assert.equal(
    cookies.pkceCodeVerifier.name,
    "__Secure-authjs.pkce.code_verifier",
  );
  assert.equal(cookies.nonce.name, "__Secure-authjs.nonce");
  // Session and CSRF cookies keep Auth.js defaults (SameSite=Lax).
  assert.equal(cookies.sessionToken, undefined);
  assert.equal(cookies.csrfToken, undefined);
  // Sign-in sets exactly the same cookies the callback reads.
  assert.deepEqual(
    apple.appleFormPostCookies("/api/auth/signin/apple", true),
    cookies,
  );
});

test("no override for other providers, other routes or plain http", () => {
  assert.equal(
    apple.appleFormPostCookies("/api/auth/callback/google", true),
    undefined,
  );
  assert.equal(
    apple.appleFormPostCookies("/api/auth/session", true),
    undefined,
  );
  assert.equal(
    apple.appleFormPostCookies("/api/auth/callback/apple", false),
    undefined,
  );
});

const ORIGINS = new Set(["https://app.comcode.test"]);

function post(pathname, headers) {
  const request = new Request(`https://app.comcode.test${pathname}`, {
    method: "POST",
    headers,
  });
  return apple.isBlockedCrossSiteAuthPost(
    request.method,
    new URL(request.url).pathname,
    guard.isCrossSiteMutation(request, ORIGINS),
  );
}

const APPLE_POST = {
  origin: "https://appleid.apple.com",
  "sec-fetch-site": "cross-site",
};
const EVIL_POST = {
  origin: "https://evil.test",
  "sec-fetch-site": "cross-site",
};

test("Apple's cross-site form_post reaches the Apple callback", () => {
  assert.equal(post("/api/auth/callback/apple", APPLE_POST), false);
  assert.equal(post("/api/auth/callback/apple", EVIL_POST), false);
});

test("every other cross-site POST to Auth.js is still blocked", () => {
  for (const p of [
    "/api/auth/callback/credentials",
    "/api/auth/callback/google",
    "/api/auth/callback/passkey",
    "/api/auth/signin/apple",
    "/api/auth/signout",
    "/api/auth/session",
  ]) {
    assert.equal(post(p, APPLE_POST), true, p);
    assert.equal(post(p, EVIL_POST), true, p);
    assert.equal(post(p, { origin: "null" }), true, p);
  }
});

test("same-origin and header-less POSTs and GETs are not affected", () => {
  assert.equal(
    post("/api/auth/callback/credentials", {
      origin: "https://app.comcode.test",
      "sec-fetch-site": "same-origin",
    }),
    false,
  );
  assert.equal(post("/api/auth/signout", {}), false);
  assert.equal(
    apple.isBlockedCrossSiteAuthPost("GET", "/api/auth/callback/google", true),
    false,
  );
});

test("the BFF still blocks cross-site POSTs, Apple's origin included", () => {
  // The Apple exception lives in the Auth.js route only, not in the BFF list.
  assert.equal(guard.isCsrfExemptPath("auth/callback/apple"), false);
  assert.equal(guard.isCsrfExemptPath("callback/apple"), false);
  const bff = new Request("https://app.comcode.test/api/v1/profiles", {
    method: "POST",
    headers: APPLE_POST,
  });
  assert.equal(guard.isCrossSiteMutation(bff, ORIGINS), true);
});
