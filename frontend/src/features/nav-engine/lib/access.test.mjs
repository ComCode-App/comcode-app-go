// Nav entry visibility by account role (TEC-948). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(path.join(here, "access.ts"), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "nav-access-"));
const compiled = path.join(tmp, "access.mjs");
fs.writeFileSync(compiled, outputText);

const { visibleNavItems } = await import(pathToFileURL(compiled).href);

const group = {
  id: "company",
  labelKey: "layout.section_company",
  accountType: "company",
  items: [
    { id: "offices", titleKey: "offices", href: "/offices" },
    {
      id: "bulk",
      titleKey: "bulk",
      href: "/bulk",
      permission: "app.company_profiles.bulk",
      accountRoles: ["owner", "admin"],
    },
  ],
};

const ids = (accountRole) =>
  visibleNavItems(group, {
    can: () => true,
    canAny: () => true,
    accountType: "company",
    accountRole,
  }).map((item) => item.id);

test("owner and admin see the bulk actions item", () => {
  assert.deepEqual(ids("owner"), ["offices", "bulk"]);
  assert.deepEqual(ids("admin"), ["offices", "bulk"]);
});

test("office managers and members do not, even with the permission", () => {
  assert.deepEqual(ids("office_manager"), ["offices"]);
  assert.deepEqual(ids("member"), ["offices"]);
});

test("an unknown role hides role-restricted items", () => {
  assert.deepEqual(ids(undefined), ["offices"]);
});
