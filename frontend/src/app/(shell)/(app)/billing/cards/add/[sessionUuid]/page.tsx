import { BillingAddCardPage } from "@/features/billing/components/billing-add-card-page";

export default async function Page({
  params,
}: {
  params: Promise<{ sessionUuid: string }>;
}) {
  const { sessionUuid } = await params;
  return <BillingAddCardPage sessionUuid={sessionUuid} />;
}
