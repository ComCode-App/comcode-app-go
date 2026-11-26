// Bank card colors on the public page (TEC-1314). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import { test } from "node:test";

import {
  bankCardColors,
  contrast,
  dominantColor,
  parseHex,
} from "./bank-color.ts";

const stops = (background) =>
  [...background.matchAll(/#[0-9a-f]{6}/g)].map((m) => parseHex(m[0]));

const rgbaOver = (css, under) => {
  const m = /rgba\((\d+), (\d+), (\d+), ([\d.]+)\)/.exec(css);
  assert.ok(m, `bad rgba ${css}`);
  const a = Number(m[4]);
  const top = { r: Number(m[1]), g: Number(m[2]), b: Number(m[3]) };
  return {
    r: top.r * a + under.r * (1 - a),
    g: top.g * a + under.g * (1 - a),
    b: top.b * a + under.b * (1 - a),
  };
};

// Seeded bank colors plus hard cases: yellow, white, black, mid grey, pastel.
const COLORS = [
  "#e11d48",
  "#1e3a8a",
  "#059669",
  "#dc2626",
  "#2563eb",
  "#facc15",
  "#ffffff",
  "#000000",
  "#808080",
  "#a5f3fc",
  "#7c3aed",
  "#f97316",
];

test("every text on a bank card keeps 4.5:1", () => {
  for (const hex of COLORS) {
    const c = bankCardColors(parseHex(hex));
    const fg = parseHex(c.foreground);
    for (const stop of stops(c.background)) {
      assert.ok(
        contrast(fg, stop) >= 4.5,
        `${hex}: text ${contrast(fg, stop)}`,
      );
      const muted = rgbaOver(c.muted, stop);
      assert.ok(
        contrast(muted, stop) >= 4.5,
        `${hex}: muted ${contrast(muted, stop)}`,
      );
      const chip = rgbaOver(c.surface, stop);
      assert.ok(
        contrast(fg, chip) >= 4.5,
        `${hex}: chip ${contrast(fg, chip)}`,
      );
    }
  }
});

test("dominant color skips white, black and grey logo pixels", () => {
  const px = [];
  const push = (r, g, b, a, n) => {
    for (let i = 0; i < n; i++) px.push(r, g, b, a);
  };
  push(255, 255, 255, 255, 400); // white background
  push(0, 0, 0, 255, 100); // black outline
  push(128, 128, 128, 255, 100); // grey text
  push(0, 0, 0, 0, 200); // transparent
  push(220, 30, 40, 255, 60); // red mark
  push(30, 60, 200, 255, 20); // small blue detail
  const c = dominantColor(px);
  assert.ok(c && c.r > 200 && c.g < 60 && c.b < 60, JSON.stringify(c));

  const mono = [];
  for (let i = 0; i < 50; i++) mono.push(0, 0, 0, 255, 255, 255, 255, 255);
  assert.equal(dominantColor(mono), null);
});
