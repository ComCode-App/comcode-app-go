"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CalendarClock,
  CreditCard,
  Gauge,
  UsersRound,
  type LucideIcon,
} from "lucide-react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { BillingSection } from "@/features/billing/components/billing-page";
import { billingService } from "@/features/billing/services/billing.service";
import { CompanySectorCard } from "@/features/company-structure";
import { PlanUsageCard } from "@/features/entitlements";
import { useEntitlements } from "@/features/entitlements/hooks/use-entitlements";
import { canManageAccount } from "@/lib/auth/types";
import { cn } from "@/lib/utils";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { AccountInvitesCard } from "./account-invites-card";

const accountKeys = {
  current: ["account", "current"] as const,
  members: ["account", "members"] as const,
};

const TABS = ["overview", "members", "subscription"] as const;
type AccountTab = (typeof TABS)[number];

const isTab = (value: string | null): value is AccountTab =>
  (TABS as readonly string[]).includes(value ?? "");

const SUBSCRIPTION_STATUSES = [
  "active",
  "trialing",
  "past_due",
  "canceled",
  "expired",
];

const ROLES: AccountRole[] = ["owner", "admin", "office_manager", "member"];
// Same check as the lead forms; the form opts out of the browser's own popup.
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function errorMessage(err: unknown, fallback: string) {
  return err instanceof Error && err.message ? err.message : fallback;
}

/**
 * Active account, its members and the subscription behind its plan on one page (TEC-1311,
 * docs/TENANCY_AND_ACCESS.md, docs/BILLING.md). The tab lives in `?tab=` so links such as
 * "upgrade" (`routes.app.billing.root`) open the subscription tab directly.
 */
export function AccountPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const tabParam = searchParams.get("tab");
  const tab: AccountTab = isTab(tabParam) ? tabParam : "overview";
  const selectTab = (next: string) => {
    const params = new URLSearchParams(searchParams.toString());
    if (next === "overview") params.delete("tab");
    else params.set("tab", next);
    const qs = params.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
  };
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

  const entitlements = useEntitlements();
  const subscriptionQuery = useQuery({
    // Same key as the subscription tab, so both share one request.
    queryKey: ["billing", "subscription", user?.account?.uuid],
    queryFn: () => billingService.getSubscription(),
  });
  const subscription = subscriptionQuery.data?.subscription ?? null;

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
  const [emailError, setEmailError] = useState<string | null>(null);
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
    if (!EMAIL_RE.test(email.trim())) {
      setEmailError(t("auth.validation.email"));
      return;
    }
    setEmailError(null);
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

          <div className="grid gap-3 sm:grid-cols-3">
            <StatTile
              icon={Gauge}
              label={t("accounts.overview.plan")}
              value={entitlements.data?.plan_name ?? "—"}
              hint={
                entitlements.data
                  ? entitlements.data.source === "free"
                    ? t("entitlements.source_free")
                    : t("entitlements.source_subscription")
                  : undefined
              }
              onClick={() => selectTab("subscription")}
            />
            <StatTile
              icon={UsersRound}
              label={t("accounts.members")}
              value={
                membersQuery.data ? String(membersQuery.data.items.length) : "—"
              }
              onClick={() => selectTab("members")}
            />
            <StatTile
              icon={subscription ? CalendarClock : CreditCard}
              label={t("accounts.overview.subscription")}
              value={
                subscription
                  ? subscription.cancel_at_period_end
                    ? t("accounts.overview.ends")
                    : SUBSCRIPTION_STATUSES.includes(subscription.status)
                      ? t(`accounts.overview.status.${subscription.status}`)
                      : subscription.status
                  : t("accounts.overview.no_subscription")
              }
              hint={
                subscription?.next_billing_at &&
                !subscription.cancel_at_period_end
                  ? t("accounts.overview.next_billing", {
                      date: new Date(
                        subscription.next_billing_at,
                      ).toLocaleDateString(locale),
                    })
                  : subscription?.cancel_at_period_end &&
                      subscription.period_end
                    ? t("accounts.overview.ends_on", {
                        date: new Date(
                          subscription.period_end,
                        ).toLocaleDateString(locale),
                      })
                    : undefined
              }
              tone={subscription?.status === "past_due" ? "warning" : undefined}
              onClick={() => selectTab("subscription")}
            />
          </div>

          <Tabs value={tab} onValueChange={selectTab} className="gap-4">
            <TabsList className="w-full sm:w-fit">
              <TabsTrigger value="overview">
                {t("accounts.tabs.overview")}
              </TabsTrigger>
              <TabsTrigger value="members">
                {t("accounts.tabs.members")}
              </TabsTrigger>
              <TabsTrigger value="subscription">
                {t("accounts.tabs.subscription")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="space-y-6">
              <PlanUsageCard />
              {isCompany ? <CompanySectorCard /> : null}
            </TabsContent>

            <TabsContent value="members" className="space-y-6">
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
                                {isCompany &&
                                (self || canEditMember(member)) ? (
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
                    noValidate
                    onSubmit={onAdd}
                    className="flex flex-col gap-3 sm:flex-row sm:items-end"
                  >
                    <label className="grid flex-1 gap-1.5 text-sm">
                      <span className="font-medium">{t("accounts.email")}</span>
                      <Input
                        type="email"
                        autoComplete="off"
                        value={email}
                        onChange={(event) => {
                          setEmail(event.target.value);
                          setEmailError(null);
                        }}
                        placeholder="name@example.com"
                        aria-invalid={emailError ? true : undefined}
                      />
                      {emailError ? (
                        <span role="alert" className="text-destructive text-xs">
                          {emailError}
                        </span>
                      ) : null}
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
            </TabsContent>

            <TabsContent value="subscription">
              <BillingSection />
            </TabsContent>
          </Tabs>
        </div>
      ) : null}
    </EntityPage>
  );
}

function StatTile({
  icon: Icon,
  label,
  value,
  hint,
  tone,
  onClick,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  hint?: string;
  tone?: "warning";
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "bg-card hover:border-foreground/30 focus-visible:ring-ring/50 flex items-start gap-3 rounded-xl border p-4 text-left transition-colors outline-none focus-visible:ring-[3px]",
        tone === "warning" && "border-amber-500/60",
      )}
    >
      <span className="bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-lg">
        <Icon className="size-4" />
      </span>
      <span className="min-w-0">
        <span className="text-muted-foreground block text-xs">{label}</span>
        <span className="block truncate font-semibold">{value}</span>
        {hint ? (
          <span
            className={cn(
              "block truncate text-xs",
              tone === "warning"
                ? "text-amber-700 dark:text-amber-400"
                : "text-muted-foreground",
            )}
          >
            {hint}
          </span>
        ) : null}
      </span>
    </button>
  );
}
