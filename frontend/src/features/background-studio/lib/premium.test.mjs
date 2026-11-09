import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const tiers = JSON.parse(
  fs.readFileSync(path.join(here, "../data/shader-tiers.json"), "utf8"),
);
const goValidate = fs.readFileSync(
  path.join(
    here,
    "../../../../../backend/internal/modules/backgrounds/model/validate.go",
  ),
  "utf8",
);

test("static shader list matches the backend (TEC-947)", () => {
  const m = /StaticShaderTypes = \[\]string\{([^}]*)\}/.exec(goValidate);
  assert.ok(m, "StaticShaderTypes not found in model/validate.go");
  const goStatic = [...m[1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
  assert.deepEqual(tiers.static, goStatic);
  const types = fs.readFileSync(
    path.join(here, "../data/shader-types.ts"),
    "utf8",
  );
  for (const id of tiers.static) {
    // Quote style is Prettier's call; accept either.
    assert.ok(
      types.includes(`'${id}'`) || types.includes(`"${id}"`),
      `${id} is not a known shader type`,
    );
  }
});
