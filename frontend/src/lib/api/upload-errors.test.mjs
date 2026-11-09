// Upload error → locale key mapping (TEC-1097). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "upload-errors-"));
// upload-errors.ts reads its keys from the central map (TEC-1110), so transpile both.
for (const name of ["upload-errors", "error-messages"]) {
  const { outputText } = ts.transpileModule(fs.readFileSync(path.join(here, `${name}.ts`), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  });
  fs.writeFileSync(
    path.join(tmp, `${name}.mjs`),
    outputText.replaceAll('"./error-messages"', '"./error-messages.mjs"'),
  );
}
const compiled = path.join(tmp, "upload-errors.mjs");

const { uploadErrorKey, UPLOAD_ERROR_KEYS } = await import(pathToFileURL(compiled).href);

const locales = Object.fromEntries(
  ["en", "tr"].map((l) => [
    l,
    JSON.parse(fs.readFileSync(path.join(here, "../../locales", l, "errors.json"), "utf8")),
  ]),
);

test("unsafe SVG and oversized uploads map to locale keys", () => {
  assert.equal(uploadErrorKey({ code: "SVG_UNSAFE_CONTENT", status: 422 }), "errors.upload_svg_unsafe");
  assert.equal(uploadErrorKey({ code: "PAYLOAD_TOO_LARGE", status: 413 }), "errors.upload_too_large");
});

test("other errors fall through to the caller's message", () => {
  assert.equal(uploadErrorKey({ code: "VALIDATION_ERROR" }), null);
  assert.equal(uploadErrorKey(new Error("boom")), null);
  assert.equal(uploadErrorKey(null), null);
  assert.equal(uploadErrorKey("SVG_UNSAFE_CONTENT"), null);
});

test("every mapped key exists in en and tr", () => {
  for (const key of Object.values(UPLOAD_ERROR_KEYS)) {
    const leaf = key.replace(/^errors\./, "");
    for (const [locale, messages] of Object.entries(locales)) {
      assert.ok(messages[leaf], `${locale} is missing ${key}`);
    }
  }
});
