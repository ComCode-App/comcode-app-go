// API error toasts and messages (TEC-1110): one failing request shows exactly one toast, and the
// message comes from the error code in the active language. errors.ts, client.ts (unwrap) and
// query/client.ts are transpiled with their app imports swapped for small stubs; react-query and
// openapi-fetch are the real packages. Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { beforeEach, test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const frontend = path.resolve(here, "../../..");
const repo = path.resolve(frontend, "..");
const localesDir = path.join(frontend, "src/locales");
// Inside node_modules so the transpiled files resolve react-query / openapi-fetch.
const cacheDir = path.join(frontend, "node_modules/.cache");
fs.mkdirSync(cacheDir, { recursive: true });
const tmp = fs.mkdtempSync(path.join(cacheDir, "api-errors-test-"));
process.on("exit", () => fs.rmSync(tmp, { recursive: true, force: true }));

const REWRITE = {
  "@/config/i18n": "./i18n-config.mjs",
  "@/lib/i18n/messages": "./messages.mjs",
  "@/config/api": "./api-config.mjs",
  "@/features/step-up-engine/lib/step-up-interceptor": "./step-up.mjs",
  "@/lib/api": "./errors.mjs",
  "./error-messages": "./error-messages.mjs",
  "./errors": "./errors.mjs",
};

function transpile(file, name) {
  let { outputText } = ts.transpileModule(fs.readFileSync(file, "utf8"), {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  });
  for (const [from, to] of Object.entries(REWRITE)) {
    outputText = outputText.replaceAll(`"${from}"`, `"${to}"`);
  }
  fs.writeFileSync(path.join(tmp, `${name}.mjs`), outputText);
}
transpile(path.join(here, "error-messages.ts"), "error-messages");
transpile(path.join(here, "errors.ts"), "errors");
transpile(path.join(here, "client.ts"), "client");
transpile(path.join(here, "../query/client.ts"), "query-client");

// --- stubs -----------------------------------------------------------------
fs.writeFileSync(
  path.join(tmp, "i18n-config.mjs"),
  `export const i18nConfig = { defaultLocale: "tr", supportedLocales: ["tr", "en"] };\n`,
);
fs.writeFileSync(
  path.join(tmp, "api-config.mjs"),
  `export const apiConfig = { baseUrl: "" };\n`,
);
fs.writeFileSync(
  path.join(tmp, "step-up.mjs"),
  `export async function runStepUpEnsure() { return false; }\n`,
);
const NAMESPACES = ["errors", "common", "auth", "entitlements", "accounts"];
const catalogs = Object.fromEntries(
  ["en", "tr"].map((locale) => [
    locale,
    Object.fromEntries(
      NAMESPACES.map((ns) => [
        ns,
        JSON.parse(
          fs.readFileSync(path.join(localesDir, locale, `${ns}.json`), "utf8"),
        ),
      ]),
    ),
  ]),
);
fs.writeFileSync(
  path.join(tmp, "messages.mjs"),
  `const catalogs = ${JSON.stringify(catalogs)};
export function translate(locale, key) {
  const [ns, ...rest] = key.split(".");
  const leaf = rest.join(".");
  return catalogs[locale]?.[ns]?.[leaf] ?? catalogs.en?.[ns]?.[leaf] ?? key;
}
`,
);

const {
  emitApiError,
  parseApiError,
  setGlobalApiErrorHandler,
  suppressApiErrorToast,
} = await import(pathToFileURL(path.join(tmp, "errors.mjs")).href);
const { unwrap } = await import(
  pathToFileURL(path.join(tmp, "client.mjs")).href
);
const { createQueryClient } = await import(
  pathToFileURL(path.join(tmp, "query-client.mjs")).href
);
const { API_ERROR_MESSAGE_KEYS, apiErrorMessageKey } = await import(
  pathToFileURL(path.join(tmp, "error-messages.mjs")).href
);

globalThis.document = { documentElement: { lang: "en" } };
const setLocale = (lang) => {
  globalThis.document.documentElement.lang = lang;
};

let toasts = [];
beforeEach(() => {
  toasts = [];
  setLocale("en");
  setGlobalApiErrorHandler((error) => toasts.push(error.message));
});

/** Toasts are queued by a macrotask; let them run. */
const flush = () => new Promise((resolve) => setTimeout(resolve, 5));

function errorBody(code, message) {
  return { success: false, error: { code, message } };
}

/** What a service does for a failing request: unwrap() the openapi-fetch result. */
function failingRequest(status, code, message = "server says no") {
  return () =>
    unwrap({
      error: errorBody(code, message),
      response: new Response(null, { status }),
    });
}

async function runMutation(options) {
  const client = createQueryClient();
  const mutation = client.getMutationCache().build(client, options);
  await mutation.execute(undefined).catch(() => {});
  await flush();
}

// --- one failing request → one toast -----------------------------------------

test("a failing query shows one toast (unwrap + QueryCache)", async () => {
  const client = createQueryClient();
  await client
    .fetchQuery({
      queryKey: ["x"],
      queryFn: failingRequest(409, "WEBHOOK_LIMIT_REACHED"),
      retry: false,
    })
    .catch(() => {});
  await flush();
  assert.deepEqual(toasts, [
    "You have reached the maximum number of webhooks.",
  ]);
});

test("a failing mutation without its own onError shows one toast (unwrap + MutationCache)", async () => {
  await runMutation({ mutationFn: failingRequest(409, "CARD_RETIRED") });
  assert.deepEqual(toasts, ["This card is already retired."]);
});

test("a mutation with its own onError reports the error itself: no global toast", async () => {
  let seen = null;
  await runMutation({
    mutationFn: failingRequest(409, "CARD_RETIRED"),
    onError: (error) => {
      seen = error.message;
    },
  });
  assert.equal(seen, "This card is already retired.");
  assert.deepEqual(toasts, []);
});

test("meta.toastErrors keeps the global toast for a cleanup-only onError", async () => {
  await runMutation({
    mutationFn: failingRequest(404, "NOT_FOUND"),
    onError: () => {},
    meta: { toastErrors: true },
  });
  assert.deepEqual(toasts, ["Record not found"]);
});

test("meta.silentErrors skips the toast for mutations and queries", async () => {
  await runMutation({
    mutationFn: failingRequest(409, "CONFLICT"),
    meta: { silentErrors: true },
  });
  const client = createQueryClient();
  await client
    .fetchQuery({
      queryKey: ["y"],
      queryFn: failingRequest(404, "NOT_FOUND"),
      retry: false,
      meta: { silentErrors: true },
    })
    .catch(() => {});
  await flush();
  assert.deepEqual(toasts, []);
});

test("a plan-limit error keeps its upgrade toast even when the caller has onError", async () => {
  await runMutation({
    mutationFn: failingRequest(403, "PLAN_LIMIT_EXCEEDED"),
    onError: () => {},
  });
  assert.equal(toasts.length, 1);
});

test("a plain await caller can opt out with suppressApiErrorToast", async () => {
  try {
    await failingRequest(409, "CONFLICT")();
  } catch (error) {
    suppressApiErrorToast(error);
  }
  await flush();
  assert.deepEqual(toasts, []);
});

test("the same error emitted twice is toasted once; step-up is never toasted", async () => {
  const error = parseApiError(409, errorBody("CONFLICT", "dup"));
  emitApiError(error);
  emitApiError(error);
  emitApiError(
    parseApiError(
      403,
      errorBody("STEP_UP_REQUIRED", "Recent authentication is required"),
    ),
  );
  await flush();
  assert.deepEqual(toasts, ["dup"]);
});

// --- code → localized message --------------------------------------------------

test("messages come from the error code in the active language", () => {
  setLocale("en");
  assert.equal(
    parseApiError(409, errorBody("LAST_SIGN_IN_METHOD", "cannot remove"))
      .message,
    "Set a password or add a passkey before removing your last sign-in method.",
  );
  assert.equal(
    parseApiError(422, errorBody("SVG_UNSAFE_CONTENT", "unsafe svg")).message,
    catalogs.en.errors.upload_svg_unsafe,
  );
  setLocale("tr");
  assert.equal(
    parseApiError(409, errorBody("LAST_SIGN_IN_METHOD", "cannot remove"))
      .message,
    catalogs.tr.auth["oauth.last_sign_in_method"],
  );
  assert.equal(
    parseApiError(429, errorBody("RATE_LIMITED", "too many requests")).message,
    "Çok fazla deneme yapıldı. Lütfen biraz bekleyin.",
  );
  assert.equal(
    parseApiError(401, errorBody("UNAUTHENTICATED", "missing token")).message,
    catalogs.tr.auth.session_expired,
  );
});

test("generic codes keep a server message that is already in the active language", () => {
  setLocale("en");
  const en = parseApiError(
    400,
    errorBody("VALIDATION_ERROR", "username is taken"),
  );
  assert.equal(en.message, "username is taken");
  assert.equal(en.serverMessage, "username is taken");
  setLocale("tr");
  assert.equal(
    parseApiError(400, errorBody("VALIDATION_ERROR", "username is taken"))
      .message,
    catalogs.tr.common.error_validation,
  );
  assert.equal(
    parseApiError(409, errorBody("CONFLICT", "Kullanıcı adı alınmış")).message,
    "Kullanıcı adı alınmış",
  );
});

test("unknown codes fall back to the server message, then to the HTTP text", () => {
  setLocale("tr");
  assert.equal(
    parseApiError(418, errorBody("SOMETHING_NEW", "teapot")).message,
    "teapot",
  );
  assert.equal(
    parseApiError(404, null).message,
    catalogs.tr.common.error_not_found,
  );
  assert.equal(parseApiError(502, {}).message, catalogs.tr.common.error_server);
  setLocale("en");
  assert.equal(
    parseApiError(418, null).message,
    catalogs.en.common.error_generic,
  );
});

test("screens can override a code's key", () => {
  const error = { code: "RATE_LIMITED" };
  assert.equal(apiErrorMessageKey(error), "errors.codes.rate_limited");
  assert.equal(
    apiErrorMessageKey(error, {
      RATE_LIMITED: "accounts.invite_page.rate_limited",
    }),
    "accounts.invite_page.rate_limited",
  );
  assert.equal(apiErrorMessageKey(new Error("x")), null);
});

// --- every backend code is mapped -----------------------------------------------

function backendErrorCodes() {
  const codes = new Set();
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (entry.name.endsWith(".go") && !entry.name.endsWith("_test.go")) {
        const src = fs.readFileSync(full, "utf8");
        // `CodeX = "X"` / `codeX = "X"` constants and literal codes passed to response helpers.
        for (const m of src.matchAll(
          /\b[cC]ode[A-Z]\w*\s*=\s*"([A-Z][A-Z0-9_]+)"/g,
        ))
          codes.add(m[1]);
        for (const m of src.matchAll(
          /response\.(?:Error|ErrorWithDetails|Conflict|BadRequest|ServiceUnavailable)\(\s*w,\s*r,\s*(?:http\.\w+,\s*)?"([A-Z][A-Z0-9_]+)"/g,
        )) {
          codes.add(m[1]);
        }
      }
    }
  };
  for (const dir of ["backend/pkg", "backend/internal"])
    walk(path.join(repo, dir));
  return codes;
}

test("every backend error code maps to a key that exists in en and tr", () => {
  const codes = backendErrorCodes();
  assert.ok(
    codes.has("VALIDATION_ERROR") && codes.has("CARD_RETIRED"),
    "code scan found nothing",
  );
  for (const code of codes) {
    const key = API_ERROR_MESSAGE_KEYS[code];
    assert.ok(
      key,
      `backend code ${code} has no entry in API_ERROR_MESSAGE_KEYS`,
    );
    const [ns, ...rest] = key.split(".");
    for (const locale of ["en", "tr"]) {
      assert.ok(
        catalogs[locale][ns]?.[rest.join(".")],
        `${locale} is missing ${key} (${code})`,
      );
    }
  }
});
