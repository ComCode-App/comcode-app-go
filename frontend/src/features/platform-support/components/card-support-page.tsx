"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Ban, CreditCard, ExternalLink, Search, UserRoundCog } from "lucide-react";
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
import { Label } from "@/components/ui/label";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import {
  type SupportCard,
  platformSupportKeys,
  platformSupportService,
} from "@/features/platform-support/services/platform-support.service";
import { Empty, Field, useDebouncedQuery } from "@/features/platform-support/components/support-ui";
import { useStepUp } from "@/features/step-up-engine";
import { isApiError } from "@/lib/api";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const ACTIVATION_STYLE: Record<SupportCard["activation_status"], string> = {
  active: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  pending: "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  retired: "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300",
};

type Action = { kind: "retire" | "reassign"; card: SupportCard };

/** Platform card support (TEC-950): find a card by serial, smart code or UID, retire or reassign it. */
export function CardSupportPage() {
  const { t } = useLocale();
  const { can } = usePermission();
  const { ensure } = useStepUp();
  const queryClient = useQueryClient();
  const { input, setInput, q } = useDebouncedQuery();
  const [action, setAction] = useState<Action | null>(null);
  const [profileUuid, setProfileUuid] = useState("");
  const canWrite = can(permissions.cards.write);

  const { data, isFetching } = useQuery({
    queryKey: platformSupportKeys.cards(q),
    queryFn: () => platformSupportService.searchCards(q),
    enabled: q.length >= 2,
  });
  const items = q.length >= 2 ? (data?.items ?? []) : [];

  const close = () => {
    setAction(null);
    setProfileUuid("");
  };
  const mutation = useMutation({
    mutationFn: async ({ kind, card }: Action) => {
      if (!(await ensure())) throw new Error(t("routes.support.step_up_cancelled"));
      return kind === "retire"
        ? platformSupportService.retireCard(card.uuid)
        : platformSupportService.reassignCard(card.uuid, profileUuid.trim());
    },
    onSuccess: (_, { kind }) => {
      void queryClient.invalidateQueries({ queryKey: platformSupportKeys.all });
      appToast.success(t(kind === "retire" ? "routes.support.card_retired" : "routes.support.card_reassigned"));
      close();
    },
    onError: (err) => appToast.error(isApiError(err) || err instanceof Error ? err.message : t("common.error_generic")),
  });

  return (
    <EntityPage
      title={t("routes.support.cards_title")}
      description={t("routes.support.cards_subtitle")}
      permission={permissions.cards.read}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        { label: t("routes.support.cards_title") },
      ]}
    >
      <div className="space-y-4 pb-12">
        <div className="bg-card rounded-xl border p-3">
          <div className="relative w-full sm:w-96">
            <Search className="text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2" />
            <Input
              placeholder={t("routes.support.cards_search")}
              aria-label={t("routes.support.cards_search")}
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
          <Empty icon={<CreditCard className="mx-auto mb-2 size-8 opacity-40" />} text={t("routes.support.no_cards")} />
        ) : (
          <ul className="space-y-3">
            {items.map((card) => (
              <li key={card.uuid}>
                <Card className="shadow-sm">
                  <CardContent className="flex flex-col gap-4 p-4 sm:flex-row sm:items-start">
                    <div className="min-w-0 flex-1 space-y-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-semibold">{card.name}</span>
                        <Badge variant="outline" className={cn("text-xs", ACTIVATION_STYLE[card.activation_status])}>
                          {t(`routes.support.activation.${card.activation_status}`)}
                        </Badge>
                        {card.is_locked ? (
                          <Badge variant="secondary" className="text-xs">
                            {t("routes.support.locked")}
                          </Badge>
                        ) : null}
                      </div>
                      <dl className="grid gap-x-6 gap-y-1 text-sm sm:grid-cols-2">
                        <Field label={t("routes.support.smart_code")} value={card.smart_code} mono />
                        <Field label={t("routes.support.serial")} value={card.card_uid || card.legacy_serial} mono />
                        <Field
                          label={t("routes.support.account")}
                          value={`${card.account.name} (${t(`routes.support.account_type.${card.account.type ?? "personal"}`)})`}
                        />
                        <Field label={t("routes.support.owner")} value={card.owner.email} />
                        <Field
                          label={t("routes.support.profile")}
                          value={card.profile ? card.profile.username || card.profile.uuid : undefined}
                        />
                        <Field
                          label={t("routes.support.order")}
                          value={card.order ? `${card.order.order_number} · ${card.order.status}` : undefined}
                        />
                        <Field
                          label={t("routes.support.route")}
                          value={card.route ? `/${card.route.path} · ${t(`routes.status.${card.route.status}`)}` : undefined}
                          mono
                        />
                      </dl>
                    </div>
                    <div className="flex shrink-0 flex-wrap items-center gap-2">
                      {card.profile?.username ? (
                        <Button asChild size="sm" variant="ghost">
                          <a href={`/${card.profile.username}`} target="_blank" rel="noreferrer">
                            <ExternalLink className="mr-1.5 size-4" />
                            {t("routes.support.open_profile")}
                          </a>
                        </Button>
                      ) : null}
                      {canWrite && card.activation_status === "active" ? (
                        <Button size="sm" variant="outline" onClick={() => setAction({ kind: "reassign", card })}>
                          <UserRoundCog className="mr-1.5 size-4" />
                          {t("routes.support.reassign")}
                        </Button>
                      ) : null}
                      {canWrite && card.route && card.activation_status !== "retired" ? (
                        <Button size="sm" variant="destructive" onClick={() => setAction({ kind: "retire", card })}>
                          <Ban className="mr-1.5 size-4" />
                          {t("routes.support.retire")}
                        </Button>
                      ) : null}
                    </div>
                  </CardContent>
                </Card>
              </li>
            ))}
          </ul>
        )}
      </div>

      <Dialog open={action !== null} onOpenChange={(open) => !open && close()}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t(action?.kind === "retire" ? "routes.support.retire_title" : "routes.support.reassign_title")}
            </DialogTitle>
            <DialogDescription>
              {t(action?.kind === "retire" ? "routes.support.retire_desc" : "routes.support.reassign_desc", {
                code: action?.card.smart_code ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          {action?.kind === "reassign" ? (
            <div className="space-y-2">
              <Label htmlFor="reassign-profile">{t("routes.support.profile_uuid")}</Label>
              <Input
                id="reassign-profile"
                value={profileUuid}
                onChange={(e) => setProfileUuid(e.target.value)}
                className="font-mono text-sm"
                placeholder="00000000-0000-0000-0000-000000000000"
              />
            </div>
          ) : null}
          <DialogFooter>
            <Button variant="outline" onClick={close}>
              {t("common.cancel")}
            </Button>
            <Button
              variant={action?.kind === "retire" ? "destructive" : "default"}
              disabled={mutation.isPending || (action?.kind === "reassign" && !UUID_RE.test(profileUuid.trim()))}
              onClick={() => action && mutation.mutate(action)}
            >
              {t(action?.kind === "retire" ? "routes.support.retire" : "routes.support.reassign")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </EntityPage>
  );
}
