// Plan limit form helpers (TEC-861). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, "limit-fields.ts"), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "limit-fields-"));
const compiled = path.join(tmp, "limit-fields.mjs");
fs.writeFileSync(compiled, outputText);

const {
  LIMIT_VALUE_PATTERN,
  limitErrorField,
  limitFieldName,
  limitsToBody,
  limitsToForm,
} = await import(pathToFileURL(compiled).href);

test("field names avoid dots inside the feature key", () => {
  assert.equal(limitFieldName("profiles.max"), "limits.profiles:max");
  assert.equal(
    limitFieldName("analytics.history_days"),
    "limits.analytics:history_days",
  );
});

test("API error fields map onto form fields", () => {
  assert.equal(limitErrorField("limits.nfc.enabled"), "limits.nfc:enabled");
  assert.equal(limitErrorField("name"), "name");
});

test("form record round-trips to the full API limit set", () => {
  const form = limitsToForm([
    { feature_key: "profiles.max", int_value: 10 },
    { feature_key: "smart_links.max", int_value: -1 },
    { feature_key: "nfc.enabled", bool_value: false },
  ]);
  assert.deepEqual(form, {
    "profiles:max": "10",
    "smart_links:max": "-1",
    "nfc:enabled": "false",
  });
  assert.deepEqual(limitsToBody({ ...form, "cards:max": "" }), [
    { feature_key: "profiles.max", int_value: 10 },
    { feature_key: "smart_links.max", int_value: -1 },
    { feature_key: "nfc.enabled", bool_value: false },
  ]);
});

test("value pattern accepts unlimited, whole numbers, bools and empty", () => {
  for (const ok of ["", "-1", "0", "42", "true", "false"]) {
    assert.ok(LIMIT_VALUE_PATTERN.test(ok), ok);
  }
  for (const bad of ["-2", "1.5", "abc", "1e3"]) {
    assert.ok(!LIMIT_VALUE_PATTERN.test(bad), bad);
  }
});
