import { PlanDetailPage } from "@/features/plans";

export default async function Page({
  params,
}: {
  params: Promise<{ uuid: string }>;
}) {
  const { uuid } = await params;
  return <PlanDetailPage uuid={uuid} />;
}
