"use client";

import {
  ArrowLeftRight,
  ExternalLink,
  Inbox,
  Lock,
  Mail,
  Phone,
  Trash2,
} from "lucide-react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useState, type ReactNode } from "react";

import {
  EntityTable,
  EntityToolbar,
  useServerListState,
} from "@/components/entity";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { useLeadMutations, useLeads } from "@/features/leads/hooks/use-leads";
import { getLeadsColumns } from "@/features/leads/components/leads-columns";
import { LeadsExportMenu } from "@/features/leads/components/leads-export-menu";
import { LeadsQuotaMeter } from "@/features/leads/components/leads-quota-meter";
import {
  leadsService,
  type Lead,
  type LeadSource,
} from "@/features/leads/services/leads.service";
import { cardsService } from "@/features/cards";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

const PAGE_SIZE = 20;

/** Inbox views: everything, unread only, or contact exchanges only (TEC-567). */
type View = "all" | "unread" | "exchanges";

const VIEW_KEYS: Record<View, string> = {
  all: "leads.filters.all",
  unread: "leads.filters.unread",
  exchanges: "leads.filters.exchanges",
};

const SOURCE_KEYS: Record<LeadSource, string> = {
  profile: "leads.source.profile",
  card: "leads.source.card",
  smart_link: "leads.source.smart_link",
  business_page: "leads.source.business_page",
};

/**
 * The active account's lead inbox (TEC-552): contact form leads and contact exchanges (TEC-567).
 * `?lead={uuid}` opens one lead (notification link). Leads over the monthly quota arrive locked
 * (TEC-941): the API masks their name and contact details, and the page says how to unlock them.
 */
export function LeadsPage() {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const { can } = usePermission();
  const { confirmDelete } = useDialogs();
  const router = useRouter();
  const searchParams = useSearchParams();
  const canWrite = can(permissions.leads.write) && canManageAccount(user);

  const [view, setView] = useState<View>("all");
  const [open, setOpen] = useState<Lead | null>(null);

  // Search and paging through the shared table state (TEC-1324).
  const listState = useServerListState({ initialPageSize: PAGE_SIZE });
  const { setPagination } = listState;
  const q = listState.params.q;
  const params = useMemo(
    () => ({
      limit: listState.params.limit,
      offset: listState.params.offset,
      q: q || undefined,
      unread: view === "unread" || undefined,
      kind: view === "exchanges" ? ("exchange" as const) : undefined,
    }),
    [listState.params.limit, listState.params.offset, q, view],
  );
  const { data, isLoading, isError, isFetching, refetch } = useLeads(params);
  const { markRead, remove } = useLeadMutations();
  const items = data?.items ?? [];
  const total = data?.total ?? 0;
  const pageCount = Math.max(
    1,
    Math.ceil(total / (listState.pagination.pageSize || PAGE_SIZE)),
  );

  // Which profile or card a lead came through, by name (the API returns their uuids).
  const profileNamesQuery = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
    staleTime: 5 * 60_000,
  });
  const cardNamesQuery = useQuery({
    queryKey: ["cards", "leads-source-names"],
    queryFn: () => cardsService.list({ page: 1, pageSize: 100 }),
    staleTime: 5 * 60_000,
  });
  const profileNames = useMemo(
    () =>
      new Map(
        (profileNamesQuery.data?.items ?? []).map((p) => [
          p.uuid,
          `${p.first_name} ${p.last_name}`.trim(),
        ]),
      ),
    [profileNamesQuery.data],
  );
  const cardNames = useMemo(
    () =>
      new Map((cardNamesQuery.data?.items ?? []).map((c) => [c.uuid, c.name])),
    [cardNamesQuery.data],
  );

  const leadParam = searchParams.get("lead");
  useEffect(() => {
    if (!leadParam) return;
    let cancelled = false;
    leadsService
      .get(leadParam)
      .then((lead) => {
        if (!cancelled) setOpen(lead);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [leadParam]);

  const fmt = (iso?: string | null) =>
    iso
      ? new Date(iso).toLocaleString(locale === "tr" ? "tr-TR" : "en-GB", {
          dateStyle: "medium",
          timeStyle: "short",
        })
      : "";

  const openLead = (lead: Lead) => {
    setOpen(lead);
    if (!lead.read_at && canWrite) markRead.mutate(lead.uuid);
  };

  const closeLead = () => {
    setOpen(null);
    if (leadParam) router.replace(routes.app.leads.root);
  };

  const deleteLead = async (lead: Lead) => {
    const ok = await confirmDelete({
      title: t("leads.delete.title"),
      description: t("leads.delete.description"),
      confirmLabel: t("leads.delete.confirm"),
      cancelLabel: t("leads.delete.cancel"),
    });
    if (!ok) return;
    remove.mutate(lead.uuid, { onSuccess: () => closeLead() });
  };

  const columns = getLeadsColumns({
    t,
    locale,
    canWrite,
    profileNames,
    cardNames,
    onOpen: openLead,
    onDelete: (lead) => void deleteLead(lead),
  });

  return (
    <div className="space-y-6 pb-12">
      <div className="flex flex-col justify-between gap-4 border-b pb-2 md:flex-row md:items-center">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-foreground flex items-center gap-2 text-2xl font-bold tracking-tight">
              <Inbox className="text-primary size-6" />
              {t("leads.title")}
            </h1>
            <Badge variant="secondary" className="font-mono text-xs">
              {total}
            </Badge>
          </div>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("leads.subtitle")}
          </p>
        </div>
        <LeadsExportMenu
          query={{
            q: q || undefined,
            unread: view === "unread" ? "true" : undefined,
            kind: view === "exchanges" ? "exchange" : undefined,
          }}
        />
      </div>

      <LeadsQuotaMeter />

      <EntityTable
        columns={columns}
        data={items}
        getRowId={(row) => row.uuid}
        onRowClick={openLead}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => void refetch()}
        emptyTitle={t("leads.empty_title")}
        emptyDescription={t("leads.empty")}
        pageCount={pageCount}
        state={listState.tableState}
        features={{
          persistKey: "app-leads-v1",
          sorting: false,
          rowSelection: false,
          columnFilters: false,
          facetedFilters: false,
        }}
        toolbarExtra={
          <div className="flex flex-wrap items-center gap-2">
            <div className="bg-muted/70 flex items-center rounded-lg border p-0.5">
              {(Object.keys(VIEW_KEYS) as View[]).map((value) => (
                <Button
                  key={value}
                  size="sm"
                  variant={view === value ? "default" : "ghost"}
                  className="h-7 rounded-md px-2.5 text-xs"
                  onClick={() => {
                    setView(value);
                    setPagination((prev) => ({ ...prev, pageIndex: 0 }));
                  }}
                >
                  {t(VIEW_KEYS[value])}
                </Button>
              ))}
            </div>
            <EntityToolbar
              onRefresh={() => void refetch()}
              refreshDisabled={isFetching}
            />
          </div>
        }
      />

      <Dialog
        open={Boolean(open)}
        onOpenChange={(value) => !value && closeLead()}
      >
        <DialogContent className="sm:max-w-lg">
          {open ? (
            <>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2">
                  {open.name}
                  {open.kind === "exchange" ? (
                    <ExchangeBadge label={t("leads.kind.exchange")} />
                  ) : null}
                  {open.locked ? (
                    <LockedBadge label={t("leads.locked.badge")} />
                  ) : null}
                </DialogTitle>
                <DialogDescription>
                  {t(SOURCE_KEYS[open.source_type])} · /{open.route_path} ·{" "}
                  {fmt(open.created_at)}
                </DialogDescription>
              </DialogHeader>
              {open.locked ? (
                <Alert>
                  <Lock className="size-4" />
                  <AlertTitle>{t("leads.locked.title")}</AlertTitle>
                  <AlertDescription className="flex flex-col gap-3">
                    <span>{t("leads.locked.description")}</span>
                    <Button asChild size="sm" className="self-start">
                      <Link href={routes.app.billing.root}>
                        {t("leads.quota.upgrade")}
                      </Link>
                    </Button>
                  </AlertDescription>
                </Alert>
              ) : null}
              <dl className="grid gap-3 text-sm">
                {open.company ? (
                  <Field label={t("leads.fields.company")}>
                    {open.company}
                  </Field>
                ) : null}
                {open.job_title ? (
                  <Field label={t("leads.fields.job_title")}>
                    {open.job_title}
                  </Field>
                ) : null}
                {open.email && open.locked ? (
                  <Field label={t("leads.fields.email")}>{open.email}</Field>
                ) : null}
                {open.phone && open.locked ? (
                  <Field label={t("leads.fields.phone")}>{open.phone}</Field>
                ) : null}
                {open.email && !open.locked ? (
                  <Field label={t("leads.fields.email")}>
                    <a
                      className="text-primary hover:underline"
                      href={`mailto:${open.email}`}
                    >
                      {open.email}
                    </a>
                  </Field>
                ) : null}
                {open.phone && !open.locked ? (
                  <Field label={t("leads.fields.phone")}>
                    <a
                      className="text-primary hover:underline"
                      href={`tel:${open.phone.replace(/[^\d+]/g, "")}`}
                    >
                      {open.phone}
                    </a>
                  </Field>
                ) : null}
                {open.message ? (
                  <Field label={t("leads.fields.message")}>
                    <p className="whitespace-pre-line">{open.message}</p>
                  </Field>
                ) : null}
                {open.sender_username ? (
                  <Field label={t("leads.fields.sender_profile")}>
                    <a
                      className="text-primary inline-flex items-center gap-1 hover:underline"
                      href={`/${encodeURIComponent(open.sender_username)}`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      {t("leads.detail.sender_profile")}
                      <ExternalLink className="size-3" />
                    </a>
                  </Field>
                ) : null}
                <Field label={t("leads.fields.consent")}>
                  {t("leads.detail.consent_given", {
                    date: fmt(open.consent_at),
                    version: open.consent_version,
                  })}
                </Field>
              </dl>
              <DialogFooter className="gap-2 sm:justify-between">
                {canWrite ? (
                  <Button
                    variant="destructive"
                    onClick={() => void deleteLead(open)}
                    disabled={remove.isPending}
                  >
                    <Trash2 className="size-4" />
                    {t("leads.actions.delete")}
                  </Button>
                ) : (
                  <span />
                )}
                <div className="flex gap-2">
                  {open.phone && !open.locked ? (
                    <Button asChild variant="outline">
                      <a href={`tel:${open.phone.replace(/[^\d+]/g, "")}`}>
                        <Phone className="size-4" />
                        {t("leads.actions.call")}
                      </a>
                    </Button>
                  ) : null}
                  {open.email && !open.locked ? (
                    <Button asChild>
                      <a href={`mailto:${open.email}`}>
                        <Mail className="size-4" />
                        {t("leads.actions.email")}
                      </a>
                    </Button>
                  ) : null}
                </div>
              </DialogFooter>
            </>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-muted-foreground text-xs">{label}</dt>
      <dd className="mt-0.5">{children}</dd>
    </div>
  );
}

function ExchangeBadge({ label }: { label: string }) {
  return (
    <Badge variant="outline" className="gap-1 text-[10px]">
      <ArrowLeftRight className="size-3" />
      {label}
    </Badge>
  );
}

function LockedBadge({ label }: { label: string }) {
  return (
    <Badge variant="secondary" className="gap-1 text-[10px]">
      <Lock className="size-3" />
      {label}
    </Badge>
  );
}
