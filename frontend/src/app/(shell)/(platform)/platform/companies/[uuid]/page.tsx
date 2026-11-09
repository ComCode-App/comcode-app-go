import { CompanyDetailPage } from "@/features/companies";

export default async function Page({
  params,
}: {
  params: Promise<{ uuid: string }>;
}) {
  const { uuid } = await params;
  return <CompanyDetailPage uuid={uuid} />;
}
