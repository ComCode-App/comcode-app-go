// Realtime notification events (TEC-1110): the `notification.created` event carries the whole
// record and its payload, so the live card shows it without a follow-up GET.
// Run with `pnpm test` (node --test).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { beforeEach, test } from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";

import ts from "typescript";

const here = path.dirname(fileURLToPath(import.meta.url));
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "live-notification-"));

const { outputText } = ts.transpileModule(
  fs.readFileSync(path.join(here, "live-notification.ts"), "utf8"),
  {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2022,
    },
  },
);
fs.writeFileSync(
  path.join(tmp, "live-notification.mjs"),
  outputText
    .replaceAll('"@/features/notifications/constants"', '"./constants.mjs"')
    .replaceAll(
      '"@/features/notifications/services/notifications.service"',
      '"./service.mjs"',
    ),
);
fs.writeFileSync(
  path.join(tmp, "constants.mjs"),
  `export function isLiveSurfaceChannel(c) { return !c || c === "inapp" || c === "realtime"; }\n`,
);
fs.writeFileSync(
  path.join(tmp, "service.mjs"),
  `export const notificationsService = {
  async get(uuid) {
    globalThis.__gets.push(uuid);
    return { uuid, channel: "realtime", status: "sent", body: "full body", payload: { from: "api" } };
  },
};
`,
);

const { enrichLiveNotification, liveNotificationFromMessage } = await import(
  pathToFileURL(path.join(tmp, "live-notification.mjs")).href
);

beforeEach(() => {
  globalThis.__gets = [];
});

// What RealtimeProvider.Deliver publishes (backend/internal/modules/notifications/providers).
const createdEvent = {
  type: "notification.created",
  data: {
    notification: {
      uuid: "n-1",
      channel: "realtime",
      title: "Export ready",
      body: "leads.csv",
      status: "queued",
      priority: "normal",
      action_url: "/platform/exports/1",
      template_code: "exports.ready",
      payload: { file: "leads.csv", rows: 12 },
    },
  },
};

test("the created event is parsed with its payload", () => {
  const live = liveNotificationFromMessage(createdEvent);
  assert.equal(live.title, "Export ready");
  assert.equal(live.template_code, "exports.ready");
  assert.deepEqual(live.payload, { file: "leads.csv", rows: 12 });
});

test("a full event is shown without fetching the notification", async () => {
  const live = liveNotificationFromMessage(createdEvent);
  assert.deepEqual(await enrichLiveNotification(live), live);
  assert.deepEqual(globalThis.__gets, []);
});

test("a partial event (no status) is still completed from the API", async () => {
  const live = liveNotificationFromMessage({
    type: "notification.created",
    data: { notification: { uuid: "n-2", title: "Hi" } },
  });
  const full = await enrichLiveNotification(live);
  assert.deepEqual(globalThis.__gets, ["n-2"]);
  assert.equal(full.body, "full body");
  assert.deepEqual(full.payload, { from: "api" });
});
