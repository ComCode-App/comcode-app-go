// Sign-in error mapping and email code parsing (TEC-1121). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, "sign-in-errors.ts"), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "sign-in-errors-"));
const compiled = path.join(tmp, "sign-in-errors.mjs");
fs.writeFileSync(compiled, outputText);

const {
  SIGN_IN_ERROR_CODES,
  isSignInErrorCode,
  normalizeEmailCode,
  signInErrorKey,
} = await import(pathToFileURL(compiled).href);

const localeDir = path.join(here, "..", "..", "..", "locales");
function hasKey(locale, key) {
  const [ns, ...rest] = key.split(".");
  const file = path.join(localeDir, locale, `${ns}.json`);
  const catalog = JSON.parse(fs.readFileSync(file, "utf8"));
  return Object.prototype.hasOwnProperty.call(catalog, rest.join("."));
}

test("sign-in codes map to their own message", () => {
  assert.equal(
    signInErrorKey("INVALID_EMAIL_CODE", "x"),
    "auth.email_code.invalid",
  );
  assert.equal(signInErrorKey("MFA_REQUIRED", "x"), "auth.totp.login_required");
  assert.equal(
    signInErrorKey("INVALID_MFA_CODE", "x"),
    "auth.totp.login_invalid",
  );
  assert.equal(
    signInErrorKey("MFA_NOT_ENROLLED", "x"),
    "auth.totp.login_not_enrolled",
  );
  assert.equal(signInErrorKey("RATE_LIMITED", "x"), "auth.login.rate_limited");
  assert.equal(
    signInErrorKey("QR_SESSION_NOT_FOUND", "x"),
    "auth.qr.exchange_failed",
  );
  assert.equal(
    signInErrorKey("QR_LOGIN_INVALID", "x"),
    "auth.qr.exchange_failed",
  );
  assert.equal(
    signInErrorKey("QR_ACCOUNT_UNAVAILABLE", "x"),
    "auth.qr.account_unavailable",
  );
  assert.equal(signInErrorKey("CredentialsSignin", "fallback"), "fallback");
  assert.equal(signInErrorKey(null, "fallback"), "fallback");
});

test("every mapped key exists in en and tr", () => {
  const keys = [
    ...SIGN_IN_ERROR_CODES.map((c) => signInErrorKey(c, "auth.login.error")),
    signInErrorKey("NO_TENANT_MEMBERSHIP", "x"),
    "auth.login.error",
    "auth.email_code.request_error",
  ];
  for (const locale of ["en", "tr"]) {
    for (const key of keys) {
      assert.ok(hasKey(locale, key), `${locale}: missing ${key}`);
    }
  }
});

test("isSignInErrorCode accepts only the pass-through codes", () => {
  assert.ok(isSignInErrorCode("INVALID_EMAIL_CODE"));
  assert.ok(!isSignInErrorCode("INVALID_CREDENTIALS"));
  assert.ok(!isSignInErrorCode(undefined));
});

test("normalizeEmailCode accepts exactly six digits", () => {
  assert.equal(normalizeEmailCode("123456"), "123456");
  assert.equal(normalizeEmailCode(" 123 456 "), "123456");
  assert.equal(normalizeEmailCode("123-456"), "123456");
  assert.equal(normalizeEmailCode("12345"), null);
  assert.equal(normalizeEmailCode("1234567"), null);
  assert.equal(normalizeEmailCode("12a456"), null);
});
