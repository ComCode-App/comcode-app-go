// Apple / Android domain association files (TEC-1183). Run with `pnpm test` (node --test).

import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

// CI runs Node 20, which cannot import .ts directly, so transpile the module first.
const here = path.dirname(fileURLToPath(import.meta.url));
const source = fs.readFileSync(
  path.join(here, "associated-domains.ts"),
  "utf8",
);
const { outputText } = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2022,
  },
});
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "associated-domains-"));
const compiled = path.join(tmp, "associated-domains.mjs");
fs.writeFileSync(compiled, outputText);

const {
  associationResponse,
  buildAppleAppSiteAssociation,
  buildAssetLinks,
  normalizeSha256Fingerprint,
  readMobileAppConfig,
} = await import(pathToFileURL(compiled).href);

const CERT =
  "14:6D:E9:83:C5:73:06:50:D8:EE:B9:95:2F:34:FC:64:16:A0:83:42:E6:1D:BE:A8:8A:04:96:B2:3F:CF:44:E5";
const CERT2 =
  "AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99:AA:BB:CC:DD:EE:FF:00:11:22:33:44:55:66:77:88:99";

function aasa(env) {
  return buildAppleAppSiteAssociation(readMobileAppConfig(env));
}
function assetlinks(env) {
  return buildAssetLinks(readMobileAppConfig(env));
}

test("defaults the iOS bundle id and Android package to KARAR-2", () => {
  const cfg = readMobileAppConfig({});
  assert.equal(cfg.iosBundleId, "software.technowide.comcode");
  assert.equal(cfg.androidPackage, "software.technowide.comcode");
  assert.deepEqual(cfg.androidSha256Certs, []);
  assert.equal(cfg.appleTeamId, "");
});

test("AASA: exact shape Apple expects", () => {
  assert.deepEqual(aasa({ MOBILE_APPLE_TEAM_ID: " ABCDE12345 " }), {
    applinks: {
      details: [
        {
          appIDs: ["ABCDE12345.software.technowide.comcode"],
          components: [{ "/": "/login/qr/*" }],
        },
      ],
    },
    webcredentials: { apps: ["ABCDE12345.software.technowide.comcode"] },
  });
});

test("AASA: custom bundle id", () => {
  const body = aasa({
    MOBILE_APPLE_TEAM_ID: "ABCDE12345",
    MOBILE_IOS_BUNDLE_ID: "com.example.app",
  });
  assert.deepEqual(body.webcredentials.apps, ["ABCDE12345.com.example.app"]);
  assert.deepEqual(body.applinks.details[0].appIDs, [
    "ABCDE12345.com.example.app",
  ]);
});

test("AASA: null (404) when the Team ID is missing or invalid", () => {
  assert.equal(aasa({}), null);
  assert.equal(aasa({ MOBILE_APPLE_TEAM_ID: "   " }), null);
  assert.equal(aasa({ MOBILE_APPLE_TEAM_ID: "abcde12345" }), null);
  assert.equal(aasa({ MOBILE_APPLE_TEAM_ID: "ABCDE1234" }), null);
  assert.equal(
    aasa({ MOBILE_APPLE_TEAM_ID: "ABCDE12345", MOBILE_IOS_BUNDLE_ID: "nodot" }),
    null,
  );
});

test("assetlinks: exact shape Google expects", () => {
  assert.deepEqual(assetlinks({ MOBILE_ANDROID_SHA256_CERTS: CERT }), [
    {
      relation: [
        "delegate_permission/common.handle_all_urls",
        "delegate_permission/common.get_login_creds",
      ],
      target: {
        namespace: "android_app",
        package_name: "software.technowide.comcode",
        sha256_cert_fingerprints: [CERT],
      },
    },
  ]);
});

test("assetlinks: several certs, normalized and de-duplicated", () => {
  const lowerNoColons = CERT.replace(/:/g, "").toLowerCase();
  const body = assetlinks({
    MOBILE_ANDROID_PACKAGE: "com.example.app",
    MOBILE_ANDROID_SHA256_CERTS: ` ${lowerNoColons} , ${CERT2},${CERT},`,
  });
  assert.equal(body[0].target.package_name, "com.example.app");
  assert.deepEqual(body[0].target.sha256_cert_fingerprints, [CERT, CERT2]);
});

test("assetlinks: null (404) when certs are missing or any is invalid", () => {
  assert.equal(assetlinks({}), null);
  assert.equal(assetlinks({ MOBILE_ANDROID_SHA256_CERTS: " , " }), null);
  assert.equal(
    assetlinks({ MOBILE_ANDROID_SHA256_CERTS: `${CERT},AB:CD` }),
    null,
  );
  // A SHA-1 (20 bytes) is not accepted.
  assert.equal(
    assetlinks({
      MOBILE_ANDROID_SHA256_CERTS:
        "DA:39:A3:EE:5E:6B:4B:0D:32:55:BF:EF:95:60:18:90:AF:D8:07:09",
    }),
    null,
  );
  assert.equal(
    assetlinks({
      MOBILE_ANDROID_SHA256_CERTS: CERT,
      MOBILE_ANDROID_PACKAGE: "1bad.package",
    }),
    null,
  );
});

test("normalizeSha256Fingerprint", () => {
  assert.equal(normalizeSha256Fingerprint(CERT.toLowerCase()), CERT);
  assert.equal(normalizeSha256Fingerprint("zz"), null);
});

test("response: 200 application/json with an hour of caching", () => {
  const res = associationResponse(aasa({ MOBILE_APPLE_TEAM_ID: "ABCDE12345" }));
  assert.equal(res.status, 200);
  assert.equal(res.headers["Content-Type"], "application/json");
  assert.equal(res.headers["Cache-Control"], "public, max-age=3600");
  assert.equal(
    JSON.parse(res.body).webcredentials.apps[0],
    "ABCDE12345.software.technowide.comcode",
  );
});

test("response: 404 when unconfigured, not cached", () => {
  const res = associationResponse(aasa({}));
  assert.equal(res.status, 404);
  assert.equal(res.headers["Cache-Control"], "no-store");
  assert.equal(res.headers["Content-Type"], "application/json");
});
