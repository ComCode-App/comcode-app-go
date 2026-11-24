// Route QR editor helpers and templates (TEC-1270). Run with `pnpm test` (node --test).
// The Go readability test decodes every template too (TestTemplatesAreReadable).

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import {
  BODY_SHAPES,
  EYE_BALL_SHAPES,
  EYE_FRAME_SHAPES,
  normalizeQrStyle,
  qrStyleProblems,
  renderQrSvg,
} from "./qr-engine.ts";
import {
  applyTemplate,
  DEFAULT_QR_STYLE,
  eyeColorMode,
  matchesTemplate,
  matrixFor,
  mirroredEyes,
  readabilityWarnings,
} from "./qr-style.ts";

const here = path.dirname(fileURLToPath(import.meta.url));
const templates = JSON.parse(
  fs.readFileSync(path.join(here, "../data/qr-templates.json"), "utf8"),
);
const locales = ["en", "tr"].map((l) =>
  JSON.parse(
    fs.readFileSync(
      path.join(here, `../../../locales/${l}/routes.json`),
      "utf8",
    ),
  ),
);
const current = {
  foreground: "#000000",
  background: "#ffffff",
  quiet_zone: 6,
  logo_profile_uuid: "00000000-0000-4000-8000-000000000001",
  logo_field: "cover",
  logo_size: 24,
  logo_mode: "overlay",
};

test("8-12 own templates, each valid and named in every locale", () => {
  assert.ok(templates.length >= 8 && templates.length <= 12);
  assert.equal(new Set(templates.map((t) => t.id)).size, templates.length);
  for (const tpl of templates) {
    const applied = applyTemplate(current, tpl);
    assert.deepEqual(qrStyleProblems(applied), [], tpl.id);
    for (const l of locales) assert.ok(l[`qr.template_${tpl.id}`], tpl.id);
  }
});

test("a template keeps the route's logo and quiet zone", () => {
  const s = normalizeQrStyle(applyTemplate(current, templates[3]));
  assert.equal(s.quiet_zone, 6);
  assert.equal(s.logo_field, "cover");
  assert.equal(s.logo_size, 24);
  assert.equal(s.logo_mode, "overlay");
  assert.ok(matchesTemplate(s, s));
  // v1 fields are re-derived, not left over from the previous design.
  const legacy = normalizeQrStyle(
    applyTemplate({ ...current, module: "dots", eye: "circle" }, templates[0]),
  );
  assert.equal(legacy.body_shape, "square");
  assert.equal(legacy.module, "square");
});

test("templates differ from each other", () => {
  const svgs = templates.map((tpl) =>
    renderQrSvg(
      ["1".repeat(21), ...Array(20).fill("10".repeat(10) + "1")],
      applyTemplate(
        { ...current, logo_profile_uuid: null, logo_field: "" },
        tpl,
      ),
      100,
    ),
  );
  assert.equal(new Set(svgs).size, svgs.length);
});

test("eye helpers", () => {
  const eyes = normalizeQrStyle(current).eyes;
  assert.equal(eyeColorMode(eyes), "inherit");
  const shared = eyes.map((e) => ({ ...e, frame_color: "#111111" }));
  assert.equal(eyeColorMode(shared), "shared");
  shared[2] = { ...shared[2], ball_color: "#222222" };
  assert.equal(eyeColorMode(shared), "per_eye");
  assert.deepEqual(
    mirroredEyes(eyes).map((e) => [e.flip_h, e.flip_v]),
    [
      [false, false],
      [true, false],
      [false, true],
    ],
  );
});

test("readability warnings and matrix choice", () => {
  const s = normalizeQrStyle({ ...current, transparent_background: true });
  assert.deepEqual(readabilityWarnings(s, [5]).sort(), [
    "large_logo",
    "logo_overlay",
    "low_margin",
    "transparent",
  ]);
  assert.deepEqual(
    readabilityWarnings(
      normalizeQrStyle({ ...current, logo_profile_uuid: null, logo_field: "" }),
      [21],
    ),
    [],
  );
  const rows = Array(21).fill("0".repeat(21));
  const big = Array(25).fill("0".repeat(25));
  assert.equal(matrixFor({ M: rows, H: big }, true), big);
  assert.equal(matrixFor({ M: rows, H: big }, false), rows);
  assert.equal(matrixFor(undefined, false), null);
});

test("every editor key exists in both locales", () => {
  const [en, tr] = locales;
  const qrKeys = (o) => Object.keys(o).filter((k) => k.startsWith("qr."));
  assert.deepEqual(qrKeys(en).sort(), qrKeys(tr).sort());
});

test("every dynamic editor key resolves (check-i18n cannot expand them)", () => {
  const keys = [
    ...BODY_SHAPES.map((v) => `shape_body_${v}`),
    ...EYE_FRAME_SHAPES.map((v) => `shape_frame_${v}`),
    ...EYE_BALL_SHAPES.map((v) => `shape_ball_${v}`),
    ...["solid", "linear", "radial"].map((v) => `fill_${v}`),
    ...[0, 1, 2].map((v) => `eye_name_${v}`),
    ...["inherit", "shared", "per_eye"].map((v) => `eye_colors_${v}`),
    ...["clean", "overlay"].flatMap((v) => [
      `logo_mode_${v}`,
      `logo_mode_${v}_hint`,
    ]),
    ...["avatar", "cover"].map((v) => `logo_${v}`),
    ...["svg", "png", "pdf"].map((v) => `format_${v}`),
    ...[
      "foreground",
      "background",
      "gradient_start",
      "gradient_end",
      "eye_frame",
      "eye_ball",
    ].map((v) => `role_${v}`),
    ...[
      "color_format",
      "inverted",
      "contrast",
      "shape",
      "quiet_zone",
      "logo",
      "gradient",
    ].map((v) => `problem_${v}`),
    ...[
      "low_margin",
      "large_logo",
      "transparent",
      "logo_overlay",
      "dense_shape",
    ].map((v) => `warning_${v}`),
  ];
  for (const l of locales)
    for (const k of keys) assert.ok(l[`qr.${k}`], `missing qr.${k}`);
});

test("default design matches qr.DefaultStyle (TEC-1273)", () => {
  const d = normalizeQrStyle(DEFAULT_QR_STYLE);
  assert.equal(d.body_shape, "smooth");
  assert.equal(d.eye_frame_shape, "rounded-large");
  assert.equal(d.eye_ball_shape, "rounded");
  assert.equal(d.foreground, "#000000");
  assert.equal(d.background, "#ffffff");
  assert.equal(d.fill_mode, "solid");
  assert.equal(d.quiet_zone, 4);
  assert.equal(d.logo_profile_uuid, null);
  assert.ok(BODY_SHAPES.includes(d.body_shape));
  assert.ok(EYE_FRAME_SHAPES.includes(d.eye_frame_shape));
  assert.ok(EYE_BALL_SHAPES.includes(d.eye_ball_shape));
  assert.deepEqual(qrStyleProblems(d), []);
});
