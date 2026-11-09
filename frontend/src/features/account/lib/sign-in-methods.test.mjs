// Last sign-in method helpers (TEC-1105). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, "sign-in-methods.ts"), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "sign-in-methods-"));
const compiled = path.join(tmp, "sign-in-methods.mjs");
fs.writeFileSync(compiled, outputText);

const {
  LAST_SIGN_IN_METHOD_MESSAGE_KEY,
  isLastSignInIdentity,
  isLastSignInMethodError,
} = await import(pathToFileURL(compiled).href);

test("the only identity of a user without password or passkey is locked", () => {
  assert.equal(
    isLastSignInIdentity({
      hasPassword: false,
      hasPasskey: false,
      identityCount: 1,
    }),
    true,
  );
});

test("another sign-in method unlocks unlinking", () => {
  assert.equal(
    isLastSignInIdentity({
      hasPassword: true,
      hasPasskey: false,
      identityCount: 1,
    }),
    false,
  );
  assert.equal(
    isLastSignInIdentity({
      hasPassword: false,
      hasPasskey: true,
      identityCount: 1,
    }),
    false,
  );
  assert.equal(
    isLastSignInIdentity({
      hasPassword: false,
      hasPasskey: false,
      identityCount: 2,
    }),
    false,
  );
});

test("detects the LAST_SIGN_IN_METHOD API error", () => {
  assert.equal(isLastSignInMethodError({ code: "LAST_SIGN_IN_METHOD" }), true);
  assert.equal(isLastSignInMethodError({ code: "CONFLICT" }), false);
  assert.equal(isLastSignInMethodError(null), false);
  assert.equal(isLastSignInMethodError(new Error("x")), false);
});

test("the message key exists in every locale", () => {
  const key = LAST_SIGN_IN_METHOD_MESSAGE_KEY.replace(/^auth\./, "");
  for (const locale of ["en", "tr"]) {
    const file = path.join(here, "../../../locales", locale, "auth.json");
    const messages = JSON.parse(fs.readFileSync(file, "utf8"));
    assert.equal(typeof messages[key], "string", `${locale}: ${key}`);
    assert.equal(typeof messages["oauth.set_password_hint"], "string", locale);
  }
});
