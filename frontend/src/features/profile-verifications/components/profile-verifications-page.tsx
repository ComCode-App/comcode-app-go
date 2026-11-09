"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BadgeCheck, Check, ExternalLink, RotateCcw, Search, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { EntityPage } from "@/components/entity";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { VerifiedBadge } from "@/features/profiles/components/verified-badge";
import {
  type ProfileVerificationRequestStatus,
  type ProfileVerificationReview,
  profileVerificationsKeys,
  profileVerificationsService,
} from "@/features/profile-verifications/services/profile-verifications.service";
import { isApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const PAGE_SIZE = 20;
const FILTERS: (ProfileVerificationRequestStatus | "all")[] = ["pending", "approved", "rejected", "revoked", "all"];

const STATUS_STYLE: Record<ProfileVerificationRequestStatus, string> = {
  pending: "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  approved: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  rejected: "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300",
  revoked: "border-border text-muted-foreground",
  cancelled: "border-border text-muted-foreground",
};

type ReasonAction = { kind: "reject" | "revoke"; item: ProfileVerificationReview };

/** Platform review queue for verified-badge requests (TEC-549). Pending first, oldest first. */
export function ProfileVerificationsPage() {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<ProfileVerificationRequestStatus | "all">("pending");
  const [search, setSearch] = useState("");
  const [q, setQ] = useState("");
  const [offset, setOffset] = useState(0);
  const [reasonAction, setReasonAction] = useState<ReasonAction | null>(null);
  const [reason, setReason] = useState("");

  useEffect(() => {
    const id = setTimeout(() => {
      setQ(search.trim());
      setOffset(0);
    }, 300);
    return () => clearTimeout(id);
  }, [search]);

  const params = useMemo(
    () => ({ limit: PAGE_SIZE, offset, q: q || undefined, status: filter === "all" ? undefined : filter }),
    [filter, offset, q],
  );
  const { data, isLoading } = useQuery({
    queryKey: profileVerificationsKeys.list(params),
    queryFn: () => profileVerificationsService.list(params),
  });
  const items = data?.items ?? [];
  const total = data?.total ?? 0;

  const done = (message: string) => {
    void queryClient.invalidateQueries({ queryKey: profileVerificationsKeys.all });
    appToast.success(message);
  };
  const onError = (err: unknown) => appToast.error(isApiError(err) ? err.message : t("common.error_generic"));

  const approve = useMutation({
    mutationFn: (uuid: string) => profileVerificationsService.approve(uuid),
    onSuccess: () => done(t("profiles.review.approved_toast")),
    onError,
  });
  const withReason = useMutation({
    mutationFn: ({ kind, item }: ReasonAction) =>
      kind === "reject"
        ? profileVerificationsService.reject(item.uuid, reason.trim())
        : profileVerificationsService.revoke(item.uuid, reason.trim()),
    onSuccess: (_, { kind }) => {
      done(t(kind === "reject" ? "profiles.review.rejected_toast" : "profiles.review.revoked_toast"));
      setReasonAction(null);
      setReason("");
    },
    onError,
  });

  const fmt = (iso?: string | null) =>
    iso
      ? new Date(iso).toLocaleString(locale === "tr" ? "tr-TR" : "en-GB", { dateStyle: "medium", timeStyle: "short" })
      : "";

  return (
    <EntityPage
      title={t("profiles.review.title")}
      description={t("profiles.review.subtitle")}
      permission={permissions.profileVerifications.manage}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("profiles.review.title") },
      ]}
    >
      <div className="space-y-4 pb-12">
        <div className="bg-card flex flex-col items-center justify-between gap-3 rounded-xl border p-3 sm:flex-row">
          <div className="relative w-full sm:w-72">
            <Search className="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
            <Input
              placeholder={t("profiles.review.search_placeholder")}
              aria-label={t("profiles.review.search_placeholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-9 pl-9 text-sm"
            />
          </div>
          <div className="bg-muted/70 flex flex-wrap items-center rounded-lg border p-0.5" role="group">
            {FILTERS.map((value) => (
              <Button
                key={value}
                size="sm"
                variant={filter === value ? "default" : "ghost"}
                aria-pressed={filter === value}
                className="h-7 rounded-md px-2.5 text-xs"
                onClick={() => {
                  setFilter(value);
                  setOffset(0);
                }}
              >
                {t(`profiles.review.filter.${value}`)}
              </Button>
            ))}
          </div>
        </div>

        {isLoading ? (
          <p className="text-muted-foreground py-10 text-center text-sm">{t("common.loading")}</p>
        ) : items.length === 0 ? (
          <Card>
            <CardContent className="text-muted-foreground py-12 text-center text-sm">
              <BadgeCheck className="mx-auto mb-2 size-8 opacity-40" />
              {t("profiles.review.empty")}
            </CardContent>
          </Card>
        ) : (
          <ul className="space-y-3">
            {items.map((item) => {
              const name = `${item.profile.first_name} ${item.profile.last_name}`.trim() || item.profile.username;
              return (
                <li key={item.uuid}>
                  <Card className="shadow-sm">
                    <CardContent className="flex flex-col gap-4 p-4 sm:flex-row sm:items-start">
                      <Avatar className="size-12 shrink-0 rounded-xl">
                        {item.profile.avatar_url ? <AvatarImage src={item.profile.avatar_url} alt="" /> : null}
                        <AvatarFallback className="rounded-xl">{name.slice(0, 1).toUpperCase() || "?"}</AvatarFallback>
                      </Avatar>
                      <div className="min-w-0 flex-1 space-y-1.5">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="font-semibold">{name}</span>
                          {item.status === "approved" ? <VerifiedBadge /> : null}
                          <Badge variant="outline" className={cn("text-xs", STATUS_STYLE[item.status])}>
                            {t(`profiles.review.status.${item.status}`)}
                          </Badge>
                          {item.origin === "re_review" ? (
                            <Badge variant="secondary" className="text-xs">
                              {t("profiles.review.origin_re_review")}
                            </Badge>
                          ) : null}
                        </div>
                        <p className="text-muted-foreground text-sm">
                          {[item.profile.title, item.profile.company_name].filter(Boolean).join(" · ") ||
                            t("profiles.review.no_organization")}
                        </p>
                        <p className="text-muted-foreground text-xs">
                          {t("profiles.review.account", {
                            name: item.account.name,
                            type: t(`profiles.review.account_type.${item.account.type}`),
                          })}
                          {item.requested_by_email ? ` · ${item.requested_by_email}` : ""} · {fmt(item.requested_at)}
                        </p>
                        {item.note ? (
                          <p className="bg-muted/40 rounded-md border px-2.5 py-1.5 text-sm">{item.note}</p>
                        ) : null}
                        {item.review_reason ? (
                          <p className="text-sm">
                            <span className="font-medium">{t("profiles.review.reason")}: </span>
                            {item.review_reason}
                          </p>
                        ) : null}
                        {item.revoke_reason ? (
                          <p className="text-sm">
                            <span className="font-medium">{t("profiles.review.revoke_reason")}: </span>
                            {item.revoke_reason}
                          </p>
                        ) : null}
                      </div>
                      <div className="flex shrink-0 flex-wrap items-center gap-2">
                        {item.profile.username ? (
                          <Button asChild size="sm" variant="ghost">
                            <a href={`/${item.profile.username}`} target="_blank" rel="noreferrer">
                              <ExternalLink className="mr-1.5 size-4" />
                              {t("profiles.review.open_page")}
                            </a>
                          </Button>
                        ) : null}
                        {item.status === "pending" ? (
                          <>
                            <Button
                              size="sm"
                              onClick={() => approve.mutate(item.uuid)}
                              disabled={approve.isPending}
                            >
                              <Check className="mr-1.5 size-4" />
                              {t("profiles.review.approve")}
                            </Button>
                            <Button size="sm" variant="outline" onClick={() => setReasonAction({ kind: "reject", item })}>
                              <X className="mr-1.5 size-4" />
                              {t("profiles.review.reject")}
                            </Button>
                          </>
                        ) : null}
                        {item.status === "approved" ? (
                          <Button size="sm" variant="outline" onClick={() => setReasonAction({ kind: "revoke", item })}>
                            <RotateCcw className="mr-1.5 size-4" />
                            {t("profiles.review.revoke")}
                          </Button>
                        ) : null}
                      </div>
                    </CardContent>
                  </Card>
                </li>
              );
            })}
          </ul>
        )}

        {total > PAGE_SIZE ? (
          <div className="flex items-center justify-end gap-2 text-sm">
            <span className="text-muted-foreground">
              {t("profiles.review.range", {
                from: offset + 1,
                to: Math.min(offset + PAGE_SIZE, total),
                total,
              })}
            </span>
            <Button size="sm" variant="outline" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>
              {t("table.go_prev")}
            </Button>
            <Button size="sm" variant="outline" disabled={offset + PAGE_SIZE >= total} onClick={() => setOffset(offset + PAGE_SIZE)}>
              {t("table.go_next")}
            </Button>
          </div>
        ) : null}
      </div>

      <Dialog
        open={reasonAction !== null}
        onOpenChange={(open) => {
          if (!open) {
            setReasonAction(null);
            setReason("");
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t(reasonAction?.kind === "revoke" ? "profiles.review.revoke_title" : "profiles.review.reject_title")}
            </DialogTitle>
            <DialogDescription>
              {t(reasonAction?.kind === "revoke" ? "profiles.review.revoke_desc" : "profiles.review.reject_desc")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="verification-reason">{t("profiles.review.reason")}</Label>
            <Textarea
              id="verification-reason"
              value={reason}
              maxLength={500}
              rows={3}
              onChange={(e) => setReason(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setReasonAction(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={!reason.trim() || withReason.isPending}
              onClick={() => reasonAction && withReason.mutate(reasonAction)}
            >
              {t(reasonAction?.kind === "revoke" ? "profiles.review.revoke" : "profiles.review.reject")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </EntityPage>
  );
}
