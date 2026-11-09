import type { Metadata } from "next";

import { InviteAcceptPage } from "@/features/accounts";
import { AuthShell } from "@/features/auth/components/auth-shell";

/** The token is in the path: never send this URL as a Referer (TEC-1104; header in next.config). */
export const metadata: Metadata = { referrer: "no-referrer" };

/** Company invite link (TEC-945). Public: signed-in and signed-out visitors both land here. */
export default async function Page({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  return (
    <AuthShell>
      <InviteAcceptPage token={token} />
    </AuthShell>
  );
}
