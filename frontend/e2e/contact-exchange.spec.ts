import { expect, test, type Page } from "@playwright/test";

import { createProfile, login, updatePublicPage } from "./support/api";
import { readRunState } from "./support/env";
import { loginAs, useEnglish } from "./support/ui";

const users = readRunState();

// Contact exchange (TEC-567): an anonymous visitor and a signed-in visitor share their details
// with a profile owner, who then finds both exchanges in the leads inbox.
test.describe.configure({ mode: "serial" });

let username = "";

test.beforeAll(async () => {
  const ownerToken = await login(
    users.exchangeOwner.email,
    users.exchangeOwner.password,
  );
  const ownerProfile = await createProfile(ownerToken, {
    firstName: "Deniz",
    lastName: "Kaya",
    email: "deniz@example.test",
  });
  username = await updatePublicPage(ownerToken, ownerProfile, {
    enabled: true,
    contact_exchange_enabled: true,
  });

  const visitorToken = await login(
    users.exchangeVisitor.email,
    users.exchangeVisitor.password,
  );
  await createProfile(visitorToken, {
    firstName: "Mert",
    lastName: "Aydın",
    company: "Beta Ltd",
    title: "CTO",
    email: "mert@example.test",
  });
});

test.beforeEach(async ({ page }) => {
  await useEnglish(page);
});

test("anonymous visitor shares their details", async ({ page }) => {
  await page.goto(`/${username}`);
  const section = exchangeSection(page);
  await section.getByRole("button", { name: "Share my details" }).click();

  await section.getByLabel("Name").fill("Selin Ak");
  await section.getByLabel("Email").fill("selin@example.test");
  await section.getByLabel("Company (optional)").fill("Gamma AŞ");
  await section.getByRole("checkbox").check();
  await section.getByRole("button", { name: "Send my details" }).click();

  await expect(section.getByText("Your details were shared")).toBeVisible();
  await expect(
    section.getByRole("link", { name: "Save Deniz Kaya to your contacts" }),
  ).toHaveAttribute("href", new RegExp(`/api/v1/public/p/${username}/vcard`));
});

test("signed-in visitor shares their ComCode profile", async ({ page }) => {
  await loginAs(page, users.exchangeVisitor);
  await page.goto(`/${username}`);
  const section = exchangeSection(page);
  await section.getByRole("button", { name: "Share my details" }).click();

  await expect(section.getByRole("combobox")).toHaveText(
    /Mert Aydın — CTO, Beta Ltd/,
  );
  await section.getByRole("checkbox").check();
  await section.getByRole("button", { name: "Share my profile" }).click();

  await expect(section.getByText("Your details were shared")).toBeVisible();
});

test("owner finds both exchanges in the leads inbox", async ({ page }) => {
  await loginAs(page, users.exchangeOwner);
  await page.goto("/leads");
  await page.getByRole("button", { name: "Contact exchanges" }).click();

  const mert = page.getByRole("button", { name: /Mert Aydın/ });
  await expect(mert).toBeVisible();
  await expect(page.getByRole("button", { name: /Selin Ak/ })).toBeVisible();
  await expect(mert).toContainText("CTO, Beta Ltd");

  await mert.click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("mert@example.test").first()).toBeVisible();
  await expect(dialog.getByText("Contact exchange")).toBeVisible();
});

function exchangeSection(page: Page) {
  return page.locator("section").filter({
    has: page.getByRole("heading", { name: "Share your details" }),
  });
}
