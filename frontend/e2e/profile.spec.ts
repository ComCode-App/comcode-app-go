import { expect, test } from "@playwright/test";

import { readRunState } from "./support/env";
import { loginAs, useEnglish } from "./support/ui";

const users = readRunState();

test("subscriber creates a profile", async ({ page }) => {
  await useEnglish(page);
  await loginAs(page, users.subscriber);

  await page.goto("/profiles/create");
  await page.locator("#field-first_name-0").fill("Ada");
  await page.getByPlaceholder("Doe").first().fill("Lovelace");
  await page.getByRole("button", { name: "Create Profile" }).click();

  await expect(page.getByText("Profile created successfully.")).toBeVisible();
  await expect(page).toHaveURL(/\/profiles\/[0-9a-f-]{36}$/);
});
