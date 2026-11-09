import { InviteAcceptPage } from "@/features/accounts";
import { AuthShell } from "@/features/auth/components/auth-shell";

/** Company invite link (TEC-945). Public: signed-in and signed-out visitors both land here. */
export default async function Page({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params;
  return (
    <AuthShell>
      <InviteAcceptPage token={token} />
    </AuthShell>
  );
}
