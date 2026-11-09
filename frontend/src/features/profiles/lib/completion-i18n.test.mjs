// Every completion suggestion the backend can return (profiles/completion, TEC-549) needs copy
// in both locales; the UI renders `t(suggestion.i18n_key)`. Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const src = path.join(here, "../../..");
const goFile = path.join(src, "../../backend/internal/modules/profiles/completion/completion.go");

test("every completion suggestion key has tr and en copy", () => {
  const go = fs.readFileSync(goFile, "utf8");
  const block = /\/\/ Rule keys[\s\S]*?const \(([\s\S]*?)\)/.exec(go);
  assert.ok(block, "rule key constants not found in completion.go");
  const keys = [...block[1].matchAll(/Key\w+\s*=\s*"([^"]+)"/g)].map((m) => m[1]);
  assert.ok(keys.length >= 8, `expected the rule keys, got ${keys}`);
  for (const locale of ["en", "tr"]) {
    const messages = JSON.parse(fs.readFileSync(path.join(src, `locales/${locale}/profiles.json`), "utf8"));
    for (const key of keys) {
      const id = `completion.suggestions.${key}`;
      assert.ok(messages[id]?.trim(), `${locale} profiles.json is missing ${id}`);
    }
  }
});
