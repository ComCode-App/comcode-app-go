"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { History, Link2, LockOpen, Search } from "lucide-react";
import { useState } from "react";

import { EntityPage } from "@/components/entity";
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
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { Empty, Field, useDebouncedQuery } from "@/features/platform-support/components/support-ui";
import {
  type SupportRoute,
  platformSupportKeys,
  platformSupportService,
} from "@/features/platform-support/services/platform-support.service";
import { useStepUp } from "@/features/step-up-engine";
import { isApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

const STATUS_STYLE: Record<SupportRoute["status"], string> = {
  active: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  reserved: "border-sky-500/40 bg-sky-500/10 text-sky-700 dark:text-sky-300",
  locked: "border-border text-muted-foreground",
  retired: "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300",
  quarantine: "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
};

function target(route: SupportRoute, t: (key: string, params?: Record<string, string | number>) => string) {
  if (route.card) return t("routes.support.card_target", { code: route.card.smart_code });
  if (route.profile) return route.profile.username ? `/${route.profile.username}` : route.profile.uuid;
  return route.direct_url || route.target_type;
}

/** Platform route support (TEC-950): find a path, see its target, status and history, release a quarantine. */
export function RouteSupportPage() {
  const { t, locale } = useLocale();
  const { can } = usePermission();
  const { ensure } = useStepUp();
  const queryClient = useQueryClient();
  const { input, setInput, q } = useDebouncedQuery();
  const [releasePath, setReleasePath] = useState<string | null>(null);
  const canWrite = can(permissions.routes.write);

  const { data, isFetching } = useQuery({
    queryKey: platformSupportKeys.routes(q),
    queryFn: () => platformSupportService.searchRoutes(q),
    enabled: q.length >= 2,
  });
  const items = q.length >= 2 ? (data?.items ?? []) : [];

  const release = useMutation({
    mutationFn: async (path: string) => {
      if (!(await ensure())) throw new Error(t("routes.support.step_up_cancelled"));
      return platformSupportService.releasePath(path);
    },
    onSuccess: (res) => {
      void queryClient.invalidateQueries({ queryKey: platformSupportKeys.all });
      appToast.success(t("routes.support.released", { path: res.path }));
      setReleasePath(null);
    },
    onError: (err) => appToast.error(isApiError(err) || err instanceof Error ? err.message : t("common.error_generic")),
  });

  const fmt = (iso?: string | null) =>
    iso ? new Date(iso).toLocaleString(locale === "tr" ? "tr-TR" : "en-GB", { dateStyle: "medium", timeStyle: "short" }) : "";

  const releaseButton = (path: string) =>
    canWrite ? (
      <Button size="sm" variant="outline" className="h-7 px-2 text-xs" onClick={() => setReleasePath(path)}>
        <LockOpen className="mr-1 size-3.5" />
        {t("routes.support.release")}
      </Button>
    ) : null;

  return (
    <EntityPage
      title={t("routes.support.routes_title")}
      description={t("routes.support.routes_subtitle")}
      permission={permissions.routes.read}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("routes.support.routes_title") },
      ]}
    >
      <div className="space-y-4 pb-12">
        <div className="bg-card rounded-xl border p-3">
          <div className="relative w-full sm:w-96">
            <Search className="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
            <Input
              placeholder={t("routes.support.routes_search")}
              aria-label={t("routes.support.routes_search")}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              className="h-9 pl-9 font-mono text-sm"
            />
          </div>
        </div>

        {q.length < 2 ? (
          <Empty icon={<Search className="mx-auto mb-2 size-8 opacity-40" />} text={t("routes.support.search_hint")} />
        ) : isFetching && items.length === 0 ? (
          <p className="text-muted-foreground py-10 text-center text-sm">{t("common.loading")}</p>
        ) : items.length === 0 ? (
          <Empty icon={<Link2 className="mx-auto mb-2 size-8 opacity-40" />} text={t("routes.support.no_routes")} />
        ) : (
          <ul className="space-y-3">
            {items.map((route) => (
              <li key={route.uuid}>
                <Card className="shadow-sm">
                  <CardContent className="space-y-3 p-4">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-sm font-semibold">/{route.path}</span>
                      <Badge variant="outline" className={cn("text-xs", STATUS_STYLE[route.status])}>
                        {t(`routes.support.status.${route.status}`)}
                      </Badge>
                      <Badge variant="secondary" className="text-xs">
                        {t(`routes.route_types.${route.route_type}`)}
                      </Badge>
                      {route.status === "quarantine" ? (
                        <>
                          <span className="text-muted-foreground text-xs">
                            {t("routes.support.quarantine_until", { date: fmt(route.quarantine_until) })}
                          </span>
                          {releaseButton(route.path)}
                        </>
                      ) : null}
                    </div>
                    <dl className="grid gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
                      <Field label={t("routes.support.target")} value={target(route, t)} />
                      <Field label={t("routes.support.owner")} value={route.owner?.email} />
                      <Field label={t("routes.support.account")} value={route.account?.name} />
                      <Field label={t("routes.support.created")} value={fmt(route.created_at)} />
                      {route.deleted_at ? <Field label={t("routes.support.deleted")} value={fmt(route.deleted_at)} /> : null}
                      {route.quarantine_released_at ? (
                        <Field label={t("routes.support.released_at")} value={fmt(route.quarantine_released_at)} />
                      ) : null}
                    </dl>
                    {route.history.length > 0 ? (
                      <div className="space-y-1.5 border-t pt-3">
                        <p className="text-muted-foreground flex items-center gap-1.5 text-xs font-medium">
                          <History className="size-3.5" />
                          {t("routes.support.history")}
                        </p>
                        <ul className="space-y-1.5">
                          {route.history.map((h, i) => (
                            <li key={`${h.created_at}-${i}`} className="flex flex-wrap items-center gap-2 text-sm">
                              <span className="font-mono text-xs">
                                /{h.old_path} → /{h.new_path}
                              </span>
                              <span className="text-muted-foreground text-xs">
                                {t(`routes.support.reason.${h.reason}`)} · {fmt(h.created_at)}
                                {h.changed_by ? ` · ${h.changed_by.email}` : ""}
                              </span>
                              {h.quarantined ? (
                                <>
                                  <Badge variant="outline" className={cn("text-xs", STATUS_STYLE.quarantine)}>
                                    {t("routes.support.quarantine_until", { date: fmt(h.quarantine_until) })}
                                  </Badge>
                                  {releaseButton(h.old_path)}
                                </>
                              ) : null}
                            </li>
                          ))}
                        </ul>
                      </div>
                    ) : null}
                  </CardContent>
                </Card>
              </li>
            ))}
          </ul>
        )}
      </div>

      <Dialog open={releasePath !== null} onOpenChange={(open) => !open && setReleasePath(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("routes.support.release_title")}</DialogTitle>
            <DialogDescription>{t("routes.support.release_desc", { path: releasePath ?? "" })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setReleasePath(null)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={release.isPending}
              onClick={() => releasePath && release.mutate(releasePath)}
            >
              {t("routes.support.release")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </EntityPage>
  );
}
