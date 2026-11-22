// QR design engine parity (TEC-1269). Run with `pnpm test` (node --test).
//
// The golden fixtures are written by the Go renderer (make qr-fixtures); the TypeScript engine
// must reproduce every SVG byte for byte, so the editor preview and browser downloads equal the
// server's /qr/render output for the same matrix.

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import {
  BODY_SHAPES,
  EYE_BALL_SHAPES,
  EYE_FRAME_SHAPES,
  fmtNum,
  normalizeQrStyle,
  qrStyleProblems,
  renderQrSvg,
  shapeIconPath,
} from "./qr-engine.ts";

const here = path.dirname(fileURLToPath(import.meta.url));
const backendQr = path.join(
  here,
  "../../../../../backend/internal/platform/qr",
);
const fixtureDir = path.join(backendQr, "testdata");
const fixtures = fs
  .readdirSync(fixtureDir)
  .filter((f) => f.endsWith(".json"))
  .map((f) => JSON.parse(fs.readFileSync(path.join(fixtureDir, f), "utf8")));

test("golden fixtures exist", () => {
  assert.ok(fixtures.length >= 20, `only ${fixtures.length} fixtures`);
});

for (const f of fixtures) {
  test(`fixture ${f.name} renders identically to Go`, () => {
    assert.deepEqual(qrStyleProblems(f.style), [], "fixture style rejected");
    assert.equal(renderQrSvg(f.matrix, f.style, f.size, f.logo_href), f.svg);
  });
}

test("shape catalogs match the Go catalogs", () => {
  const goShapes = fs.readFileSync(path.join(backendQr, "shapes.go"), "utf8");
  const list = (name) => {
    const m = new RegExp(`var ${name} = \\[\\]string\\{([^}]*)\\}`).exec(
      goShapes,
    );
    assert.ok(m, `${name} not found in shapes.go`);
    return [...m[1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
  };
  assert.deepEqual([...BODY_SHAPES], list("BodyShapes"));
  assert.deepEqual([...EYE_FRAME_SHAPES], list("EyeFrameShapes"));
  assert.deepEqual([...EYE_BALL_SHAPES], list("EyeBallShapes"));
});

test("fixtures cover every shape and color mode", () => {
  const seen = new Set();
  for (const f of fixtures) {
    const s = normalizeQrStyle(f.style);
    seen.add(`body:${s.body_shape}`);
    seen.add(`frame:${s.eye_frame_shape}`);
    seen.add(`ball:${s.eye_ball_shape}`);
    seen.add(`fill:${s.fill_mode}`);
    if (s.transparent_background) seen.add("transparent");
    if (s.logo_profile_uuid) seen.add(`logo:${s.logo_mode}`);
  }
  const need = [
    ...BODY_SHAPES.map((v) => `body:${v}`),
    ...EYE_FRAME_SHAPES.map((v) => `frame:${v}`),
    ...EYE_BALL_SHAPES.map((v) => `ball:${v}`),
    "fill:solid",
    "fill:linear",
    "fill:radial",
    "transparent",
    "logo:clean",
    "logo:overlay",
  ];
  assert.deepEqual(
    need.filter((k) => !seen.has(k)),
    [],
  );
});

test("v1 styles normalize to their v2 shapes", () => {
  const s = normalizeQrStyle({
    foreground: "#1F2937",
    background: "#FFFFFF",
    module: "dots",
    eye: "rounded",
    quiet_zone: 4,
    logo_profile_uuid: null,
    logo_field: "",
  });
  assert.equal(s.body_shape, "circle");
  assert.equal(s.eye_frame_shape, "rounded");
  assert.equal(s.eye_ball_shape, "rounded");
  assert.equal(s.foreground, "#1f2937");
  assert.equal(s.fill_mode, "solid");
  assert.equal(s.logo_mode, "clean");
  assert.equal(s.logo_size, 15);
  const v2 = normalizeQrStyle({
    foreground: "#000000",
    background: "#ffffff",
    body_shape: "leaf",
    quiet_zone: 4,
  });
  assert.equal(v2.module, "square");
  assert.equal(v2.eye_frame_shape, "square");
});

test("validation mirrors the API contrast rules", () => {
  const ok = { foreground: "#000000", background: "#ffffff", quiet_zone: 4 };
  assert.deepEqual(qrStyleProblems(ok), []);
  const kinds = (s) => qrStyleProblems({ ...ok, ...s }).map((p) => p.kind);
  assert.deepEqual(kinds({ foreground: "#ffffff", background: "#000000" }), [
    "inverted",
  ]);
  // Each gradient color is checked on its own.
  assert.deepEqual(
    kinds({
      fill_mode: "linear",
      gradient_start: "#000000",
      gradient_end: "#cccccc",
    }),
    ["contrast"],
  );
  // Eye colors are checked too; a transparent background is judged as white.
  const eyes = [
    { flip_h: false, flip_v: false, frame_color: "#eeeeee", ball_color: "" },
    { flip_h: false, flip_v: false, frame_color: "", ball_color: "" },
    { flip_h: false, flip_v: false, frame_color: "", ball_color: "" },
  ];
  assert.deepEqual(kinds({ eyes }), ["contrast"]);
  assert.deepEqual(
    kinds({ transparent_background: true, background: "#000000" }),
    [],
  );
  assert.deepEqual(kinds({ logo_size: 31 }), ["logo"]);
  assert.deepEqual(kinds({ body_shape: "triangle" }), ["shape"]);
});

test("number format is deterministic", () => {
  assert.equal(fmtNum(0.1 + 0.2), "0.3");
  assert.equal(fmtNum(-0.0004), "0");
  assert.equal(fmtNum(2.0005), "2.001");
  assert.equal(fmtNum(-2.0005), "-2.001");
  assert.equal(fmtNum(12), "12");
});

test("every shape has an icon path", () => {
  for (const s of BODY_SHAPES) assert.match(shapeIconPath("body", s), /^M/);
  for (const s of EYE_FRAME_SHAPES)
    assert.match(shapeIconPath("frame", s), /^M/);
  for (const s of EYE_BALL_SHAPES) assert.match(shapeIconPath("ball", s), /^M/);
});
