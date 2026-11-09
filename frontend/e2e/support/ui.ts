import { expect, type Page } from "@playwright/test";

import type { TestUser } from "./env";

/** Pins the UI language to English (the client reads `app.locale` from localStorage). */
export async function useEnglish(page: Page) {
  await page.addInitScript(() => {
    window.localStorage.setItem("app.locale", "en");
  });
}

export async function fillCredentials(
  page: Page,
  user: TestUser,
  loginPath = "/login",
) {
  await page.goto(loginPath);
  await page.locator("#email").fill(user.email);
  await page.locator("#password").fill(user.password);
  await page.getByRole("button", { name: "Login", exact: true }).click();
}

export async function loginAs(
  page: Page,
  user: TestUser,
  loginPath = "/login",
) {
  await fillCredentials(page, user, loginPath);
  await expect(page).not.toHaveURL(/\/login/);
}
