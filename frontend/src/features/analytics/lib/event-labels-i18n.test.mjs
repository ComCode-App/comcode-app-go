// Every event definition seeded by a migration needs a readable label in both locales; the
// platform analytics pages render `analytics.event_type.<key>` (TEC-856). Run with `pnpm test`.

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const src = path.join(here, "../../..");
const migrations = path.join(src, "../../backend/migrations");

function seededEventDefinitions() {
  const out = new Map();
  for (const file of fs
    .readdirSync(migrations)
    .filter((f) => f.endsWith(".up.sql"))) {
    const sql = fs.readFileSync(path.join(migrations, file), "utf8");
    for (const block of sql.matchAll(
      /INSERT INTO event_definitions\s*\([^)]*\)\s*VALUES([\s\S]*?);/g,
    )) {
      for (const row of block[1].matchAll(
        /\(\s*'([a-z0-9._-]+)',\s*'[^']*',\s*'[^']*',\s*'([a-z0-9_]+)'/g,
      )) {
        out.set(row[1], row[2]);
      }
    }
  }
  return out;
}

test("every seeded event type and category has tr and en labels", () => {
  const seeded = seededEventDefinitions();
  assert.ok(
    seeded.size >= 15,
    `expected the seeded event definitions, got ${[...seeded.keys()]}`,
  );
  for (const locale of ["en", "tr"]) {
    const messages = JSON.parse(
      fs.readFileSync(
        path.join(src, `locales/${locale}/analytics.json`),
        "utf8",
      ),
    );
    for (const [key, category] of seeded) {
      assert.ok(
        messages[`event_type.${key}`]?.trim(),
        `${locale} analytics.json is missing event_type.${key}`,
      );
      assert.ok(
        messages[`event_category.${category}`]?.trim(),
        `${locale} analytics.json is missing event_category.${category}`,
      );
    }
  }
});
