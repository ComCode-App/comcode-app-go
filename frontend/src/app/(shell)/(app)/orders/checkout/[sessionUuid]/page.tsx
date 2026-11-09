import { OrderCheckoutPage } from "@/features/orders/components/order-checkout-page";

export default async function Page({
  params,
}: {
  params: Promise<{ sessionUuid: string }>;
}) {
  const { sessionUuid } = await params;
  return <OrderCheckoutPage sessionUuid={sessionUuid} />;
}
