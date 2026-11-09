import crypto from "node:crypto";

import {
  createUser,
  enableTotp,
  login,
  roleUUID,
  setLocale,
} from "./support/api";
import {
  adminEmail,
  adminPassword,
  writeRunState,
  type TestUser,
} from "./support/env";
import { totpCode } from "./support/totp";

// Fresh users per run: unique emails keep runs independent (free-plan quotas,
// one-active-card limit, per-email login rate limits) without resetting the DB.
export default async function globalSetup() {
  const run = `${Date.now().toString(36)}${crypto.randomBytes(2).toString("hex")}`;
  const newUser = (kind: string): TestUser => ({
    email: `e2e-${kind}-${run}@example.test`,
    password: `E2e-${crypto.randomBytes(6).toString("hex")}A1`,
    name: "E2E",
    surname: kind.charAt(0).toUpperCase() + kind.slice(1),
  });

  const adminToken = await login(adminEmail, adminPassword);
  const subscriberRole = await roleUUID(adminToken, "subscriber");
  const superAdminRole = await roleUUID(adminToken, "super_admin");

  const subscriber = newUser("subscriber");
  const mfaSubscriber = newUser("mfa");
  const operator = newUser("operator");
  const exchangeOwner = newUser("owner");
  const exchangeVisitor = newUser("visitor");

  await createUser(adminToken, subscriber, [subscriberRole]);
  await createUser(adminToken, mfaSubscriber, [subscriberRole]);
  await createUser(adminToken, operator, [superAdminRole]);
  await createUser(adminToken, exchangeOwner, [subscriberRole]);
  await createUser(adminToken, exchangeVisitor, [subscriberRole]);

  for (const user of [subscriber, operator, exchangeOwner, exchangeVisitor]) {
    await setLocale(await login(user.email, user.password), "en");
  }
  const mfaToken = await login(mfaSubscriber.email, mfaSubscriber.password);
  await setLocale(mfaToken, "en");
  mfaSubscriber.totpSecret = await enableTotp(mfaToken, (secret) =>
    totpCode(secret),
  );

  writeRunState({
    subscriber,
    mfaSubscriber,
    operator,
    exchangeOwner,
    exchangeVisitor,
  });
}
