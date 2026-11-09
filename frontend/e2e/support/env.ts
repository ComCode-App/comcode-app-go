import fs from "node:fs";
import path from "node:path";

// Go API, used only for test setup (users, TOTP). The browser under test still talks to /api/v1.
export const apiURL = (
  process.env.E2E_API_URL ??
  process.env.API_URL ??
  "http://127.0.0.1:8080/v1"
).replace(/\/$/, "");

export const adminEmail = process.env.E2E_ADMIN_EMAIL ?? "admin@example.com";
export const adminPassword = process.env.E2E_ADMIN_PASSWORD ?? "Password1";

export function databaseURL(): string {
  if (process.env.E2E_DATABASE_URL) return process.env.E2E_DATABASE_URL;
  const user = process.env.DB_USER ?? "app";
  const password = encodeURIComponent(process.env.DB_PASSWORD ?? "change_me");
  const host = process.env.DB_HOST ?? "127.0.0.1";
  const port = process.env.DB_PORT ?? "5432";
  const name = process.env.DB_NAME ?? "app";
  const sslmode = process.env.DB_SSLMODE ?? "disable";
  return `postgres://${user}:${password}@${host}:${port}/${name}?sslmode=${sslmode}`;
}

export type TestUser = {
  email: string;
  password: string;
  name: string;
  surname: string;
  totpSecret?: string;
};

export type RunState = {
  subscriber: TestUser;
  mfaSubscriber: TestUser;
  operator: TestUser;
  /** Contact exchange (TEC-567): the profile owner and a visitor with a profile of their own. */
  exchangeOwner: TestUser;
  exchangeVisitor: TestUser;
};

const statePath = path.join(__dirname, "..", ".state", "run.json");

export function writeRunState(state: RunState) {
  fs.mkdirSync(path.dirname(statePath), { recursive: true });
  fs.writeFileSync(statePath, JSON.stringify(state, null, 2));
}

export function readRunState(): RunState {
  return JSON.parse(fs.readFileSync(statePath, "utf8")) as RunState;
}
