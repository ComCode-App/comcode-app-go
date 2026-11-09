import { Client } from "pg";

import { databaseURL } from "./env";

export type ProvisionedCard = { activation_code: string; smart_code: string };

/**
 * Activation codes are never returned by the order or card APIs (only by the
 * step-up protected manifest export), so the test reads them from the database.
 * Provisioning after "paid" runs in the background; poll until the card exists.
 */
export async function waitForProvisionedCard(
  orderNumber: string,
  timeoutMs = 30_000,
): Promise<ProvisionedCard> {
  const client = new Client({ connectionString: databaseURL() });
  await client.connect();
  try {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const { rows } = await client.query<ProvisionedCard>(
        `SELECT nc.activation_code, nc.smart_code
           FROM nfc_cards nc
           JOIN orders o ON o.id = nc.order_id
          WHERE o.order_number = $1 AND nc.activation_code IS NOT NULL`,
        [orderNumber],
      );
      if (rows.length > 0) return rows[0];
      await new Promise((r) => setTimeout(r, 500));
    }
    throw new Error(
      `order ${orderNumber}: no provisioned card after ${timeoutMs} ms`,
    );
  } finally {
    await client.end();
  }
}
