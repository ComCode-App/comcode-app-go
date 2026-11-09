// Slug preview helper (TEC-1111). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, "slugify.ts"), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "slugify-"));
const compiled = path.join(tmp, "slugify.mjs");
fs.writeFileSync(compiled, outputText);

const { slugify } = await import(pathToFileURL(compiled).href);

const cases = [
  ["Çağrı Şirketi", "cagri-sirketi"],
  ["İSTANBUL Ofis", "istanbul-ofis"],
  ["Güneş Ölçü", "gunes-olcu"],
  ["Işık", "isik"],
  ["ÇĞİÖŞÜ", "cgiosu"],
  ["Café Crème", "cafe-creme"],
  ["Hello World", "hello-world"],
  ["  -Hello-  ", "hello"],
  ["a  --  b__c", "a-b-c"],
  ["Route 66", "route-66"],
  ["", ""],
];

for (const [input, want] of cases) {
  test(`slugify(${JSON.stringify(input)}) -> ${JSON.stringify(want)}`, () => {
    assert.equal(slugify(input), want);
  });
}

test("custom separator (role slugs use underscores)", () => {
  assert.equal(
    slugify("Çağrı Merkezi Yöneticisi", "_"),
    "cagri_merkezi_yoneticisi",
  );
  assert.equal(slugify("__Support Agent__", "_"), "support_agent");
});
