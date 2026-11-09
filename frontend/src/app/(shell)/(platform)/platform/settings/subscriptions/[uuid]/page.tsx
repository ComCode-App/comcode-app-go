import { SubscriptionDetailPage } from "@/features/subscriptions";

export default async function Page({
  params,
}: {
  params: Promise<{ uuid: string }>;
}) {
  const { uuid } = await params;
  return <SubscriptionDetailPage uuid={uuid} />;
}
