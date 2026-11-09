import { expect, test } from "@playwright/test";

import { readRunState } from "./support/env";
import { totpCode } from "./support/totp";
import { fillCredentials, loginAs, useEnglish } from "./support/ui";

const users = readRunState();

test.beforeEach(async ({ page }) => {
  await useEnglish(page);
});

test("subscriber signs in with email and password", async ({ page }) => {
  await loginAs(page, users.subscriber);
  await expect(page).toHaveURL(/\/$/);
});

test("wrong password is rejected", async ({ page }) => {
  await fillCredentials(page, {
    ...users.subscriber,
    password: "Wrong-password-1",
  });
  await expect(page.getByText("Invalid email or password")).toBeVisible();
  await expect(page).toHaveURL(/\/login/);
});

test("user with an authenticator must enter a TOTP code", async ({ page }) => {
  const user = users.mfaSubscriber;
  await fillCredentials(page, user);

  await expect(page.getByText("Authenticator required")).toBeVisible();
  await page.locator("#totp_code").fill(totpCode(user.totpSecret!));
  await page.getByRole("button", { name: "Verify and sign in" }).click();

  await expect(page).not.toHaveURL(/\/login/);
  await expect(page).toHaveURL(/\/$/);
});

test("platform operator lands on the platform shell", async ({ page }) => {
  await loginAs(page, users.operator, "/platform/login");
  await expect(page).toHaveURL(/\/platform(\/|$)/);
});
