import { BillingCheckoutPage } from "@/features/billing/components/billing-checkout-page";

export default async function Page({
  params,
}: {
  params: Promise<{ sessionUuid: string }>;
}) {
  const { sessionUuid } = await params;
  return <BillingCheckoutPage sessionUuid={sessionUuid} />;
}
