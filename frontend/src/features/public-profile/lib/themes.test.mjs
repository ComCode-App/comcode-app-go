// Public profile theme catalog checks (TEC-601). Run with `pnpm test` (node --test).
//
// Every text/surface pair a theme produces must reach WCAG AA (4.5:1). Translucent surfaces
// (the glass theme) are checked over black, white and the theme's own backdrop, because a
// Background Studio design can put anything behind them.

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const themes = JSON.parse(
  fs.readFileSync(path.join(here, "themes.json"), "utf8"),
);
const MIN = 4.5;

function parse(hex) {
  const m = /^#([0-9a-f]{6})([0-9a-f]{2})?$/i.exec(hex);
  assert.ok(m, `bad color ${hex}`);
  const n = parseInt(m[1], 16);
  return {
    r: (n >> 16) & 255,
    g: (n >> 8) & 255,
    b: n & 255,
    a: m[2] ? parseInt(m[2], 16) / 255 : 1,
  };
}

/** Paints `top` over the opaque color `under`. */
function over(top, under) {
  const a = top.a;
  return {
    r: top.r * a + under.r * (1 - a),
    g: top.g * a + under.g * (1 - a),
    b: top.b * a + under.b * (1 - a),
    a: 1,
  };
}

function luminance({ r, g, b }) {
  const ch = (v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * ch(r) + 0.7152 * ch(g) + 0.0722 * ch(b);
}

export function contrast(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

const BLACK = parse("#000000");
const WHITE = parse("#ffffff");

for (const [id, theme] of Object.entries(themes)) {
  test(`theme ${id} keeps text contrast >= ${MIN}:1`, () => {
    const c = Object.fromEntries(
      Object.entries(theme.colors).map(([k, v]) => [k, parse(v)]),
    );
    const backdrops =
      c.card.a < 1
        ? [BLACK, WHITE, parse(theme.backdropBase)]
        : [parse(theme.backdropBase)];
    for (const backdrop of backdrops) {
      const card = over(c.card, backdrop);
      const surface = (name) => over(c[name], card);
      const pairs = [
        ["foreground", c.foreground, "card", card],
        ["foreground", c.foreground, "background", surface("background")],
        ["foreground", c.foreground, "muted", surface("muted")],
        ["mutedForeground", c.mutedForeground, "card", card],
        ["mutedForeground", c.mutedForeground, "muted", surface("muted")],
        ["primary", c.primary, "card", card],
        [
          "primaryForeground",
          c.primaryForeground,
          "primary",
          surface("primary"),
        ],
        [
          "primaryForeground",
          c.primaryForeground,
          "primaryEnd",
          surface("primaryEnd"),
        ],
        ["accentForeground", c.accentForeground, "accent", surface("accent")],
      ];
      for (const [fgName, fg, bgName, bg] of pairs) {
        const ratio = contrast(over(fg, bg), bg);
        assert.ok(
          ratio >= MIN,
          `${id}: ${fgName} on ${bgName} is ${ratio.toFixed(2)}:1 (backdrop ${JSON.stringify(backdrop)})`,
        );
      }
    }
  });
}

test("catalog matches the backend allow-lists", () => {
  const goModel = fs.readFileSync(
    path.join(
      here,
      "../../../../../backend/internal/modules/profiles/model/public.go",
    ),
    "utf8",
  );
  const list = (name) => {
    const m = new RegExp(`${name} = \\[\\]string\\{([^}]*)\\}`).exec(goModel);
    assert.ok(m, `${name} not found in model/public.go`);
    return [...m[1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
  };
  assert.deepEqual(Object.keys(themes), list("PublicThemes"));
  // Premium themes and their basic fallbacks (TEC-947) match model.PremiumThemes.
  const m = /PremiumThemes = map\[string\]string\{([^}]*)\}/.exec(goModel);
  assert.ok(m, "PremiumThemes not found in model/public.go");
  const goPremium = Object.fromEntries(
    [...m[1].matchAll(/"([^"]+)":\s*"([^"]+)"/g)].map((x) => [x[1], x[2]]),
  );
  const premium = Object.fromEntries(
    Object.entries(themes)
      .filter(([, theme]) => theme.premium)
      .map(([id, theme]) => [id, theme.fallback]),
  );
  assert.deepEqual(premium, goPremium);
  for (const [id, fallback] of Object.entries(premium)) {
    assert.ok(
      themes[fallback] && !themes[fallback].premium,
      `${id}: fallback ${fallback} must be a basic theme`,
    );
  }
  const layouts = list("PublicLayoutModes");
  for (const [id, theme] of Object.entries(themes)) {
    assert.ok(
      layouts.includes(theme.layout),
      `${id}: unknown layout ${theme.layout}`,
    );
  }
});
