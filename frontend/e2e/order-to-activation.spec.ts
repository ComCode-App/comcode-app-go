import { expect, test, type Page } from "@playwright/test";

import { waitForProvisionedCard, type ProvisionedCard } from "./support/db";
import { readRunState } from "./support/env";
import { loginAs, useEnglish } from "./support/ui";

const users = readRunState();

// One order travels the whole chain: subscriber orders → operator marks it paid
// in the pipeline (provisions a card) → subscriber activates the card → /c/{code} resolves.
test.describe.configure({ mode: "serial" });

let orderNumber = "";
let card: ProvisionedCard;

test.beforeEach(async ({ page }) => {
  await useEnglish(page);
});

test("subscriber places a card order", async ({ page }) => {
  await loginAs(page, users.subscriber);
  await page.goto("/orders");

  await page.getByRole("button", { name: "Order New Card" }).first().click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Order New NFC Card")).toBeVisible();

  // Step 1: the first product and variant are preselected.
  await dialog.getByRole("button", { name: "Next" }).click();
  // Step 2: customisation.
  await dialog.locator("#printName").fill("E2E Card Holder");
  await dialog.getByRole("button", { name: "Next" }).click();
  // Step 3: shipping.
  await dialog.locator("#shipFullName").fill("E2E Card Holder");
  await dialog.locator("#shipPhone").fill("+905550000000");
  await dialog.locator("#shipAddress").fill("E2E Street 1");
  await dialog.locator("#shipCity").fill("Istanbul");

  const created = page.waitForResponse(
    (res) =>
      res.request().method() === "POST" && /\/api\/v1\/orders$/.test(res.url()),
  );
  await dialog.getByRole("button", { name: "Confirm & Place Order" }).click();
  const res = await created;
  expect(res.status()).toBe(201);
  orderNumber = (await res.json()).data.order_number;

  await expect(page.getByText("Order created.")).toBeVisible();
  await expect(page.getByText(orderNumber)).toBeVisible();
});

test("operator marks the order paid in the pipeline", async ({ page }) => {
  expect(orderNumber).not.toBe("");
  await loginAs(page, users.operator, "/platform/login");
  await page.goto("/platform/orders/pipeline");

  const orderCard = pipelineCard(page, orderNumber);
  await orderCard.getByRole("button", { name: "Mark as Paid" }).click();
  await expect(page.getByText("Order status updated.")).toBeVisible();

  // "paid" orders sit in the design/logo pending column and advance to production next.
  await expect(
    pipelineCard(page, orderNumber).getByRole("button", {
      name: "Send to Production",
    }),
  ).toBeVisible();

  card = await waitForProvisionedCard(orderNumber);
  expect(card.activation_code).toMatch(/^ACT-/);
});

test("subscriber activates the provisioned card", async ({ page }) => {
  expect(card).toBeDefined();

  // Before activation the card route is reserved and shows the card placeholder.
  await page.goto(`/c/${card.smart_code}`);
  await expect(page.getByText("ComCode Smart Business Card")).toBeVisible();

  await loginAs(page, users.subscriber);
  await page.goto("/cards");
  await page.getByRole("button", { name: "Activate Card" }).first().click();

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Activate New NFC Card")).toBeVisible();
  await dialog.locator("#activation_code").fill(card.activation_code);
  await dialog.getByRole("button", { name: "Activate Card" }).click();

  await expect(
    page.getByText("NFC card activated and linked to your account!"),
  ).toBeVisible();

  // The card route is now active.
  const resolved = await page.request.get(`/api/v1/c/${card.smart_code}`);
  expect(resolved.ok()).toBeTruthy();
  expect((await resolved.json()).data).toMatchObject({ is_active: true });
});

function pipelineCard(page: Page, number: string) {
  return page
    .locator('[data-slot="card"]')
    .filter({ has: page.getByText(number, { exact: true }) });
}
