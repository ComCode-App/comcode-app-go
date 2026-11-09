// Guard against double API error toasts (TEC-1117). A failed request already queues the global
// toast (unwrap → emitApiError), so a catch block that shows its own toast must decide which one
// the user sees:
//   - call suppressApiErrorToast(error) and keep its own (more specific) text, or
//   - toast only non-API errors: `if (!isApiError(error)) toast…` / `!(error instanceof ApiError)`, or
//   - when the try block cannot throw an ApiError (clipboard, JSON.parse, raw downloads), say so
//     with a `toast-guard:` comment inside the catch block.
// The scan covers `try/catch` and `.catch(callback)` in every src file. Run with `pnpm test`.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const src = path.resolve(here, "../..");

const TOAST_CALL = /^(app)?toast\b/i;
const NON_API_GUARD =
  /!\s*isApiError\(|!\s*\(\s*[\w.]+\s+instanceof\s+ApiError\b/;
const MARKER = "toast-guard:";

function isToastCall(node, sf) {
  return (
    ts.isCallExpression(node) && TOAST_CALL.test(node.expression.getText(sf))
  );
}

/** True when `node` sits in the then-branch of an `if (!isApiError(…))` inside `root`. */
function guardedAgainstApiErrors(node, root, sf) {
  for (let cur = node; cur && cur !== root; cur = cur.parent) {
    const parent = cur.parent;
    if (
      parent &&
      ts.isIfStatement(parent) &&
      parent.thenStatement === cur &&
      NON_API_GUARD.test(parent.expression.getText(sf))
    ) {
      return true;
    }
  }
  return false;
}

/** Problems in one handler body (catch block or .catch callback). */
function checkHandler(body, sf) {
  const text = body.getFullText(sf);
  if (text.includes(MARKER) || /\bsuppressApiErrorToast\(/.test(text)) {
    return false;
  }
  let bad = false;
  const visit = (node) => {
    if (bad) return;
    // A nested function is not run by this handler.
    if (node !== body && ts.isFunctionLike(node)) return;
    if (isToastCall(node, sf) && !guardedAgainstApiErrors(node, body, sf)) {
      bad = true;
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(body);
  return bad;
}

/** `file:line` of every catch handler that toasts without handling the global API toast. */
export function findDoubleToasts(fileName, source) {
  const sf = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const found = [];
  const report = (node) =>
    found.push(
      `${fileName}:${sf.getLineAndCharacterOfPosition(node.getStart(sf)).line + 1}`,
    );
  const visit = (node) => {
    if (ts.isCatchClause(node) && checkHandler(node.block, sf)) report(node);
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      node.expression.name.text === "catch" &&
      node.arguments[0] &&
      ts.isFunctionLike(node.arguments[0]) &&
      checkHandler(node.arguments[0], sf)
    ) {
      report(node.expression.name);
    }
    ts.forEachChild(node, visit);
  };
  visit(sf);
  return found;
}

function sourceFiles(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name !== "generated") sourceFiles(full, out);
    } else if (/\.tsx?$/.test(entry.name) && !entry.name.endsWith(".d.ts")) {
      out.push(full);
    }
  }
  return out;
}

test("the scan flags a catch that toasts an API error without suppressing it", () => {
  const flagged = (code) => findDoubleToasts("x.tsx", code).length > 0;
  assert.ok(
    flagged(`async function f() {
      try { await api(); } catch { appToast.error("failed"); }
    }`),
  );
  assert.ok(
    flagged(`async function f() {
      try { await api(); } catch (e) {
        if (isApiError(e)) toast.error(e.message); else toast.error("x");
      }
    }`),
  );
  assert.ok(flagged(`api().catch(() => { appToast.error("failed"); });`));

  assert.ok(
    !flagged(`async function f() {
      try { await api(); } catch (e) { suppressApiErrorToast(e); appToast.error("x"); }
    }`),
  );
  assert.ok(
    !flagged(`async function f() {
      try { await api(); } catch (e) { if (!isApiError(e)) toast.error("x"); }
    }`),
  );
  assert.ok(
    !flagged(`async function f() {
      try { await api(); } catch (e) {
        if (!(e instanceof ApiError)) { toast.error("x"); }
      }
    }`),
  );
  assert.ok(
    !flagged(`async function f() {
      try { await copy(); } catch {
        // toast-guard: clipboard only
        toast.error("x");
      }
    }`),
  );
  assert.ok(
    !flagged(`async function f() {
      try { await api(); } catch { setError("inline"); }
    }`),
  );
});

test("no catch block shows its own toast on top of the global API error toast", () => {
  const offenders = sourceFiles(src).flatMap((file) =>
    findDoubleToasts(
      path.relative(src, file).split(path.sep).join("/"),
      fs.readFileSync(file, "utf8"),
    ),
  );
  assert.deepEqual(
    offenders,
    [],
    "Call suppressApiErrorToast(error) and keep the component's text, toast only when " +
      "!isApiError(error), or add a `toast-guard:` comment when no API call can fail there.",
  );
});
