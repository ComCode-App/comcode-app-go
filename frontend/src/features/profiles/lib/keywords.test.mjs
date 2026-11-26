// Profile keywords stored in the translation note (TEC-1304). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import { test } from "node:test";

import {
  KEYWORD_MAX_LENGTH,
  KEYWORDS_MAX_COUNT,
  addKeywords,
  joinKeywords,
  parseKeywords,
} from "./keywords.ts";

test("parseKeywords splits, trims and de-duplicates", () => {
  assert.deepEqual(parseKeywords(""), []);
  assert.deepEqual(parseKeywords(null), []);
  assert.deepEqual(parseKeywords(" Bulut ,  güvenlik;DevOps\n\nbulut "), [
    "Bulut",
    "güvenlik",
    "DevOps",
  ]);
  assert.deepEqual(parseKeywords("İstanbul, istanbul"), ["İstanbul"]);
});

test("an old free-text note becomes a single keyword per line", () => {
  assert.deepEqual(parseKeywords("Toplantılar için\nhafta içi arayın"), [
    "Toplantılar için",
    "hafta içi arayın",
  ]);
});

test("joinKeywords round-trips through parseKeywords", () => {
  const keywords = ["NFC", "Kartvizit", "Yazılım"];
  assert.equal(joinKeywords(keywords), "NFC, Kartvizit, Yazılım");
  assert.deepEqual(parseKeywords(joinKeywords(keywords)), keywords);
});

test("addKeywords appends pasted lists and enforces limits", () => {
  assert.deepEqual(addKeywords(["NFC"], "a, b ,nfc"), ["NFC", "a", "b"]);
  const long = "x".repeat(KEYWORD_MAX_LENGTH + 10);
  assert.equal(addKeywords([], long)[0].length, KEYWORD_MAX_LENGTH);
  const many = Array.from(
    { length: KEYWORDS_MAX_COUNT + 5 },
    (_, i) => `k${i}`,
  );
  assert.equal(addKeywords([], many.join(",")).length, KEYWORDS_MAX_COUNT);
});

test("addKeywords never shortens an existing keyword", () => {
  const oldNote =
    "Resmi iş iletişimi ve teknik ortaklıklar için öncelikli kanallardan ulaşabilirsiniz.";
  assert.deepEqual(addKeywords(parseKeywords(oldNote), "NFC"), [
    oldNote,
    "NFC",
  ]);
});
