import type { Metadata } from "next";

import { ExchangeDelete } from "@/features/leads/components/exchange-delete";

type PageProps = { searchParams: Promise<{ token?: string }> };

export const metadata: Metadata = { robots: { index: false, follow: false } };

/**
 * KVKK "delete my data" page of the contact exchange visitor copy email (TEC-940). Opening the
 * link only shows the page; the exchange is deleted when the visitor confirms, so mail scanners
 * that open links cannot use the single-use token up.
 */
export default async function ExchangeDeletePage({ searchParams }: PageProps) {
  const { token } = await searchParams;
  return (
    <ExchangeDelete
      token={typeof token === "string" ? token.slice(0, 200) : ""}
    />
  );
}
