"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Loading } from "@/components/common/loading";
import { EntitySectionCard } from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Permission } from "@/config/permissions";
import {
  type AccountInvite,
  type AccountRole,
  accountService,
} from "@/features/accounts/services/account.service";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const inviteKeys = { list: ["account", "invites"] as const };

const ROLES: AccountRole[] = ["owner", "admin", "office_manager", "member"];

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error && err.message ? err.message : fallback;
}

/**
 * Email invites of the active company account (TEC-945): invite any address with a role, list
 * open invites (expired ones marked), resend and revoke. Owners/admins with the invite
 * permissions only; the caller decides whether to render it.
 */
export function AccountInvitesCard({ isOwner }: { isOwner: boolean }) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const { confirmDelete } = useDialogs();
  const queryClient = useQueryClient();
  const canRead = Boolean(user?.permissions.includes(Permission.AppAccountInvitesRead));
  const canWrite = Boolean(user?.permissions.includes(Permission.AppAccountInvitesWrite));

  const invitesQuery = useQuery({
    queryKey: inviteKeys.list,
    queryFn: accountService.invites,
    enabled: canRead,
  });
  const invalidate = () => void queryClient.invalidateQueries({ queryKey: inviteKeys.list });
  const onError = (err: unknown) => appToast.error(errorMessage(err, t("common.error_generic")));

  const [email, setEmail] = useState("");
  const [role, setRole] = useState<AccountRole>("member");
  const create = useMutation({
    mutationFn: accountService.createInvite,
    onSuccess: () => {
      setEmail("");
      setRole("member");
      invalidate();
      appToast.success(t("accounts.invites.toast.sent"));
    },
    onError,
  });
  const resend = useMutation({
    mutationFn: accountService.resendInvite,
    onSuccess: () => {
      invalidate();
      appToast.success(t("accounts.invites.toast.resent"));
    },
    onError,
  });
  const revoke = useMutation({
    mutationFn: accountService.revokeInvite,
    onSuccess: () => {
      invalidate();
      appToast.success(t("accounts.invites.toast.revoked"));
    },
    onError,
  });

  if (!canRead && !canWrite) return null;

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (!email.trim()) return;
    create.mutate({ email: email.trim(), role });
  };

  const confirmRevoke = async (invite: AccountInvite) => {
    const confirmed = await confirmDelete({
      title: t("accounts.invites.revoke_title"),
      confirmLabel: t("accounts.invites.revoke"),
      description: t("accounts.invites.revoke_description", { email: invite.email }),
    });
    if (confirmed) revoke.mutate(invite.uuid);
  };

  const formatDate = (value: string) =>
    new Date(value).toLocaleDateString(locale === "tr" ? "tr-TR" : "en-US", { dateStyle: "medium" });
  const canTouch = (invite: AccountInvite) => canWrite && (isOwner || invite.role !== "owner");
  const invites = invitesQuery.data?.items ?? [];

  return (
    <EntitySectionCard title={t("accounts.invites.title")}>
      <div className="space-y-4">
        {canWrite ? (
          <form onSubmit={onSubmit} className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label className="grid flex-1 gap-1.5 text-sm">
              <span className="font-medium">{t("accounts.email")}</span>
              <Input
                type="email"
                autoComplete="off"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="name@example.com"
              />
            </label>
            <label className="grid gap-1.5 text-sm">
              <span className="font-medium">{t("accounts.role")}</span>
              <Select value={role} onValueChange={(value) => setRole(value as AccountRole)}>
                <SelectTrigger className="w-44">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ROLES.filter((r) => isOwner || r !== "owner").map((r) => (
                    <SelectItem key={r} value={r}>
                      {t(`accounts.roles.${r}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </label>
            <Button type="submit" disabled={create.isPending || !email.trim()}>
              {t("accounts.invites.send")}
            </Button>
          </form>
        ) : null}
        <p className="text-muted-foreground text-xs">{t("accounts.invites.hint")}</p>

        {invitesQuery.isLoading ? <Loading label={t("common.loading")} /> : null}
        {canRead && invitesQuery.data ? (
          invites.length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("accounts.invites.empty")}</p>
          ) : (
            <div className="overflow-x-auto rounded-md border">
              <table className="w-full text-sm">
                <thead className="bg-muted/50 border-b">
                  <tr>
                    <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                      {t("accounts.email")}
                    </th>
                    <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                      {t("accounts.role")}
                    </th>
                    <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                      {t("accounts.invites.status")}
                    </th>
                    <th className="h-10 px-4" />
                  </tr>
                </thead>
                <tbody>
                  {invites.map((invite) => (
                    <tr key={invite.uuid} className="hover:bg-muted/50 border-b last:border-0">
                      <td className="p-4">
                        <div className="font-medium">{invite.email}</div>
                        <div className="text-muted-foreground text-xs">
                          {invite.source === "import"
                            ? t("accounts.invites.from_import")
                            : invite.invited_by_name}
                        </div>
                      </td>
                      <td className="p-4">{t(`accounts.roles.${invite.role}`)}</td>
                      <td className="p-4">
                        <Badge variant={invite.status === "expired" ? "danger" : "secondary"}>
                          {t(`accounts.invites.statuses.${invite.status}`)}
                        </Badge>
                        <div className="text-muted-foreground mt-1 text-xs">
                          {invite.status === "expired"
                            ? t("accounts.invites.expired_on", { date: formatDate(invite.expires_at) })
                            : t("accounts.invites.expires_on", { date: formatDate(invite.expires_at) })}
                        </div>
                      </td>
                      <td className="p-4 text-right whitespace-nowrap">
                        {canTouch(invite) ? (
                          <>
                            <Button
                              size="sm"
                              variant="ghost"
                              disabled={resend.isPending}
                              onClick={() => resend.mutate(invite.uuid)}
                            >
                              {t("accounts.invites.resend")}
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              disabled={revoke.isPending}
                              onClick={() => void confirmRevoke(invite)}
                            >
                              {t("accounts.invites.revoke")}
                            </Button>
                          </>
                        ) : null}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )
        ) : null}
      </div>
    </EntitySectionCard>
  );
}
