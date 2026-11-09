"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import {
  EntityHeader,
  EntityPage,
  EntitySectionCard,
} from "@/components/entity";
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
import { routes } from "@/config/routes";
import {
  type AccountMember,
  type AccountRole,
  accountService,
  isInvitedResult,
} from "@/features/accounts/services/account.service";
import { CompanySectorCard } from "@/features/company-structure";
import { PlanUsageCard } from "@/features/entitlements";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { AccountInvitesCard } from "./account-invites-card";

const accountKeys = {
  current: ["account", "current"] as const,
  members: ["account", "members"] as const,
};

const ROLES: AccountRole[] = ["owner", "admin", "office_manager", "member"];

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error && err.message ? err.message : fallback;
}

/** Active account settings and members (docs/TENANCY_AND_ACCESS.md). */
export function AccountPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const { confirmDelete } = useDialogs();
  const queryClient = useQueryClient();

  const accountQuery = useQuery({
    queryKey: accountKeys.current,
    queryFn: accountService.current,
  });
  const membersQuery = useQuery({
    queryKey: accountKeys.members,
    queryFn: accountService.members,
  });

  const account = accountQuery.data;
  const isCompany = account?.type === "company";
  const canManage = canManageAccount(user);
  const isOwner = account?.role === "owner";

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: accountKeys.current });
    void queryClient.invalidateQueries({ queryKey: accountKeys.members });
  };
  const onError = (err: unknown) =>
    appToast.error(errorMessage(err, t("common.error_generic")));

  const [email, setEmail] = useState("");
  const [role, setRole] = useState<AccountRole>("member");
  const addMember = useMutation({
    mutationFn: accountService.addMember,
    onSuccess: (result) => {
      setEmail("");
      setRole("member");
      invalidate();
      if (isInvitedResult(result)) {
        // Not a verified user: an invite went to the address instead (TEC-1104).
        void queryClient.invalidateQueries({
          queryKey: ["account", "invites"],
        });
        appToast.success(
          t("accounts.toast.member_invited", { email: result.invite.email }),
        );
        return;
      }
      appToast.success(t("accounts.toast.member_added"));
    },
    onError,
  });
  const changeRole = useMutation({
    mutationFn: ({ userUuid, role }: { userUuid: string; role: AccountRole }) =>
      accountService.changeRole(userUuid, role),
    onSuccess: () => {
      invalidate();
      appToast.success(t("accounts.toast.member_updated"));
    },
    onError,
  });
  const removeMember = useMutation({
    mutationFn: accountService.removeMember,
    onSuccess: (_data, userUuid) => {
      if (userUuid === user?.uuid) {
        // Left the account: the session falls back to the personal account.
        window.location.assign(routes.app.home);
        return;
      }
      invalidate();
      appToast.success(t("accounts.toast.member_removed"));
    },
    onError,
  });

  const onAdd = (event: FormEvent) => {
    event.preventDefault();
    if (!email.trim()) return;
    addMember.mutate({ email: email.trim(), role });
  };

  const confirmRemove = async (member: AccountMember) => {
    const self = member.user_uuid === user?.uuid;
    const confirmed = await confirmDelete({
      title: self ? t("accounts.leave_title") : t("accounts.remove_title"),
      confirmLabel: self ? t("accounts.leave") : t("accounts.remove"),
      description: self
        ? t("accounts.leave_description", { name: account?.name ?? "" })
        : t("accounts.remove_description", {
            name: `${member.name} ${member.surname}`.trim(),
          }),
    });
    if (confirmed) removeMember.mutate(member.user_uuid);
  };

  const canEditMember = (member: AccountMember) =>
    canManage &&
    member.user_uuid !== user?.uuid &&
    (isOwner || member.role !== "owner");

  return (
    <EntityPage
      title={t("accounts.title")}
      description={t("accounts.subtitle")}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("accounts.title") },
      ]}
    >
      {accountQuery.isLoading ? <Loading label={t("common.loading")} /> : null}
      {accountQuery.isError ? (
        <ErrorState
          title={t("common.error_generic")}
          onRetry={() => void accountQuery.refetch()}
          retryLabel={t("common.retry")}
        />
      ) : null}

      {account ? (
        <div className="space-y-6">
          <EntityHeader
            title={account.name}
            description={isCompany ? account.slug : t("accounts.personal_hint")}
            badges={
              <>
                <Badge variant="outline">
                  {t(`accounts.types.${account.type}`)}
                </Badge>
                <Badge variant="secondary">
                  {t(`accounts.roles.${account.role}`)}
                </Badge>
              </>
            }
          />

          <PlanUsageCard />

          {isCompany ? <CompanySectorCard /> : null}

          <EntitySectionCard title={t("accounts.members")}>
            {membersQuery.isLoading ? (
              <Loading label={t("common.loading")} />
            ) : null}
            {membersQuery.data ? (
              <div className="overflow-x-auto rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/50 border-b">
                    <tr>
                      <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                        {t("accounts.member")}
                      </th>
                      <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                        {t("accounts.role")}
                      </th>
                      <th className="h-10 px-4" />
                    </tr>
                  </thead>
                  <tbody>
                    {membersQuery.data.items.map((member) => {
                      const self = member.user_uuid === user?.uuid;
                      return (
                        <tr
                          key={member.user_uuid}
                          className="hover:bg-muted/50 border-b last:border-0"
                        >
                          <td className="p-4">
                            <div className="font-medium">
                              {member.name} {member.surname}
                              {self ? (
                                <span className="text-muted-foreground ml-1 text-xs">
                                  ({t("accounts.you")})
                                </span>
                              ) : null}
                            </div>
                            <div className="text-muted-foreground text-xs">
                              {member.email}
                            </div>
                          </td>
                          <td className="p-4">
                            {canEditMember(member) ? (
                              <Select
                                value={member.role}
                                disabled={changeRole.isPending}
                                onValueChange={(value) =>
                                  changeRole.mutate({
                                    userUuid: member.user_uuid,
                                    role: value as AccountRole,
                                  })
                                }
                              >
                                <SelectTrigger
                                  className="h-8 w-44"
                                  aria-label={t("accounts.role")}
                                >
                                  <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                  {ROLES.filter(
                                    (r) => isOwner || r !== "owner",
                                  ).map((r) => (
                                    <SelectItem key={r} value={r}>
                                      {t(`accounts.roles.${r}`)}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            ) : (
                              t(`accounts.roles.${member.role}`)
                            )}
                          </td>
                          <td className="p-4 text-right">
                            {isCompany && (self || canEditMember(member)) ? (
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={removeMember.isPending}
                                onClick={() => void confirmRemove(member)}
                              >
                                {self
                                  ? t("accounts.leave")
                                  : t("accounts.remove")}
                              </Button>
                            ) : null}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            ) : null}
          </EntitySectionCard>

          {isCompany && canManage ? (
            <AccountInvitesCard isOwner={isOwner} />
          ) : null}

          {isCompany && canManage ? (
            <EntitySectionCard title={t("accounts.add_member")}>
              <form
                onSubmit={onAdd}
                className="flex flex-col gap-3 sm:flex-row sm:items-end"
              >
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
                  <Select
                    value={role}
                    onValueChange={(value) => setRole(value as AccountRole)}
                  >
                    <SelectTrigger className="w-44">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {ROLES.filter((r) => isOwner || r !== "owner").map(
                        (r) => (
                          <SelectItem key={r} value={r}>
                            {t(`accounts.roles.${r}`)}
                          </SelectItem>
                        ),
                      )}
                    </SelectContent>
                  </Select>
                </label>
                <Button
                  type="submit"
                  disabled={addMember.isPending || !email.trim()}
                >
                  {t("accounts.add_member")}
                </Button>
              </form>
              <p className="text-muted-foreground mt-2 text-xs">
                {t("accounts.add_member_hint")}
              </p>
            </EntitySectionCard>
          ) : null}
        </div>
      ) : null}
    </EntityPage>
  );
}
