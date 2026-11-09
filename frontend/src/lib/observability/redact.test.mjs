// Log redaction + NextAuth error logging (TEC-1113). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "redact-"));
for (const name of ["redact", "auth-logger"]) {
  let { outputText } = ts.transpileModule(
    fs.readFileSync(path.join(here, `${name}.ts`), "utf8"),
    {
      compilerOptions: {
        module: ts.ModuleKind.ESNext,
        target: ts.ScriptTarget.ES2022,
      },
    },
  );
  outputText = outputText.replaceAll(
    '"@/lib/observability/redact"',
    '"./redact.mjs"',
  );
  fs.writeFileSync(path.join(tmp, `${name}.mjs`), outputText);
}
const { redactString, describeError } = await import(
  pathToFileURL(path.join(tmp, "redact.mjs")).href
);
const { logAuthError, logCredentialsFailure } = await import(
  pathToFileURL(path.join(tmp, "auth-logger.mjs")).href
);

function captureConsole(fn) {
  const lines = [];
  const orig = { error: console.error, warn: console.warn };
  console.error = (...a) =>
    lines.push([
      "error",
      a.map((x) => (typeof x === "string" ? x : JSON.stringify(x))).join(" "),
    ]);
  console.warn = (...a) =>
    lines.push([
      "warn",
      a.map((x) => (typeof x === "string" ? x : JSON.stringify(x))).join(" "),
    ]);
  try {
    fn();
  } finally {
    Object.assign(console, orig);
  }
  return lines;
}

test("redactString removes emails, tokens, secrets and phones", () => {
  const cases = [
    ["no user ayse.k+1@example.com.tr", ["ayse.k", "example.com"], "[email]"],
    ["jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.sig-1 bad", ["eyJ"], "[token]"],
    ["Authorization: Bearer abc.def-123", ["abc.def-123"], "Bearer [token]"],
    ['{"password":"hunter2"}', ["hunter2"], "[redacted]"],
    ["refresh_token=r3fr3sh&x=1", ["r3fr3sh"], "refresh_token=[redacted]"],
    ["sms +90 532 123 45 67", ["532 123"], "[phone]"],
  ];
  for (const [input, bad, want] of cases) {
    const got = redactString(input);
    for (const b of bad) assert.ok(!got.includes(b), `${got} still has ${b}`);
    assert.ok(got.includes(want), `${got} lacks ${want}`);
  }
  assert.equal(
    redactString("upstream unavailable (502)"),
    "upstream unavailable (502)",
  );
});

test("redactString removes PEM private keys (TEC-1123)", () => {
  const cases = [
    "-----BEGIN PRIVATE KEY-----\nMIGTAgEAMBMG\nAgEG\n-----END PRIVATE KEY-----",
    '{"private_key":"-----BEGIN PRIVATE KEY-----\\nMIGTAgEAMBMG\\n-----END PRIVATE KEY-----"}',
    "cut -----BEGIN EC PRIVATE KEY-----\nMIGTAgEAMBMG",
  ];
  for (const input of cases) {
    const out = redactString(input);
    assert.ok(!out.includes("MIGT"), out);
    assert.ok(out.includes("[private-key]"), out);
  }
});

test("describeError includes the cause and is redacted", () => {
  const err = new Error("fetch failed", {
    cause: new Error("connect to a@b.io refused"),
  });
  const text = describeError(err);
  assert.match(
    text,
    /fetch failed \| cause: Error: connect to \[email\] refused/,
  );
});

test("logCredentialsFailure never logs email or password", () => {
  const err = Object.assign(
    new Error("Email or password is incorrect for veli@example.com"),
    {
      code: "INVALID_CREDENTIALS",
      status: 401,
    },
  );
  const lines = captureConsole(() => logCredentialsFailure(err));
  assert.equal(lines.length, 1);
  const [, text] = lines[0];
  assert.match(text, /INVALID_CREDENTIALS/);
  assert.match(text, /401/);
  assert.ok(!text.includes("veli"), text);
  assert.ok(!text.includes("example.com"), text);

  const noCode = captureConsole(() =>
    logCredentialsFailure(new Error("timeout for veli@example.com")),
  );
  assert.ok(!noCode[0][1].includes("veli@example.com"), noCode[0][1]);
});

test("logAuthError redacts and downgrades expected errors", () => {
  const expected = Object.assign(
    new Error("Read more at https://errors.authjs.dev"),
    { type: "CredentialsSignin" },
  );
  const unexpected = Object.assign(new Error("adapter failed"), {
    type: "AdapterError",
    cause: { err: new Error("lookup of zeynep@example.com failed") },
  });
  const lines = captureConsole(() => {
    logAuthError(expected);
    logAuthError(unexpected);
  });
  assert.equal(lines[0][0], "warn");
  assert.equal(lines[1][0], "error");
  assert.match(
    lines[1][1],
    /\[auth\]\[error\] AdapterError: adapter failed \| cause: Error: lookup of \[email\] failed/,
  );
  assert.ok(!lines[1][1].includes("zeynep"));
});
