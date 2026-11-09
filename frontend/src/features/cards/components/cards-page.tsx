"use client";

import React, { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import type { Updater, PaginationState } from "@tanstack/react-table";
import {
  CreditCard,
  Plus,
  KeyRound,
  LayoutGrid,
  Table as TableIcon,
  Search,
  Sparkles,
  Wifi,
  Lock,
  ShoppingBag,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { QuotaNotice, useQuotaReached } from "@/features/entitlements";
import { useLocale } from "@/providers/locale-provider";
import {
  cardsService,
  type CardDetail,
  type CardSummary,
} from "../services/cards.service";
import { cardsKeys } from "../hooks/query-keys";
import { useCardMutations } from "../hooks/use-cards";
import { InteractiveNFCCard } from "./InteractiveNFCCard";
import { CardActivationDialog } from "./card-activation-dialog";
import { CardSettingsDialog } from "./card-settings-dialog";
import { createCardsColumns } from "./cards-columns";
import { SubscriberOrderDialog } from "@/features/orders";
import { RouteQrDialog } from "@/features/route-qr";
import { EntityTable } from "@/components/entity";

export function CardsPage() {
  const { t } = useLocale();
  const cardQuotaReached = useQuotaReached("cards.max");
  const [viewMode, setViewMode] = useState<"grid" | "table">("grid");
  const [searchTerm, setSearchTerm] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // Dialog states
  const [activationOpen, setActivationOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [qrOpen, setQrOpen] = useState(false);
  const [orderOpen, setOrderOpen] = useState(false);
  const [selectedCard, setSelectedCard] = useState<
    CardSummary | CardDetail | null
  >(null);

  const { toggleLockMutation } = useCardMutations();

  const { data, isLoading, refetch } = useQuery({
    queryKey: cardsKeys.list({ page, pageSize, search: searchTerm }),
    queryFn: () => cardsService.list({ page, pageSize, search: searchTerm }),
  });

  const cards = data?.items || [];
  const total = data?.total || 0;

  const activeCount = cards.filter((c) => c.is_active && !c.is_locked).length;
  const lockedCount = cards.filter((c) => c.is_locked || !c.is_active).length;

  // Cmd+K hits link to /cards?open={uuid} (TEC-951).
  useEffect(() => {
    const url = new URL(window.location.href);
    const uuid = url.searchParams.get("open");
    if (!uuid) return;
    url.searchParams.delete("open");
    window.history.replaceState(null, "", url.pathname + url.search);
    cardsService
      .get(uuid)
      .then((detail) => {
        setSelectedCard(detail);
        setSettingsOpen(true);
      })
      .catch(() => {});
  }, []);

  const handleOpenSettings = (card: CardSummary | null) => {
    setSelectedCard(card);
    setSettingsOpen(true);
  };

  const handleOpenQR = (card: CardSummary) => {
    setSelectedCard(card);
    setQrOpen(true);
  };

  const handleToggleLock = (card: CardSummary) => {
    toggleLockMutation.mutate(card.uuid);
  };

  const tableColumns = createCardsColumns({
    t,
    onSettingsClick: handleOpenSettings,
    onQRClick: handleOpenQR,
    onToggleLock: handleToggleLock,
  });

  return (
    <div className="space-y-6 pb-12">
      {/* Page Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 pb-2 border-b">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
              <CreditCard className="w-6 h-6 text-primary" />
              {t("cards.page_title")}
            </h1>
            <Badge variant="secondary" className="font-mono text-xs">
              {total} {t("cards.total_count_badge")}
            </Badge>
          </div>
          <p className="text-sm text-muted-foreground mt-1">
            {t("cards.page_description")}
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <Button
            variant="outline"
            className="gap-2"
            onClick={() => setOrderOpen(true)}
          >
            <ShoppingBag className="w-4 h-4 text-primary" />
            {t("cards.order_card_btn")}
          </Button>

          <Button
            variant="outline"
            className="gap-2"
            onClick={() => setActivationOpen(true)}
            disabled={cardQuotaReached}
          >
            <KeyRound className="w-4 h-4 text-primary" />
            {t("cards.activate_card_btn")}
          </Button>

          <Button
            variant="default"
            className="gap-2 shadow-sm"
            onClick={() => handleOpenSettings(null)}
            disabled={cardQuotaReached}
          >
            <Plus className="w-4 h-4" />
            {t("cards.create_card_btn")}
          </Button>
        </div>
      </div>

      <QuotaNotice feature="cards.max" />

      {/* KPI Overview Strip */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <Card className="bg-card/60 backdrop-blur-sm">
          <CardContent className="p-4 flex items-center justify-between">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground font-medium">{t("cards.kpi.total_cards")}</p>
              <p className="text-2xl font-bold text-foreground">{total}</p>
            </div>
            <div className="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
              <CreditCard className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-card/60 backdrop-blur-sm">
          <CardContent className="p-4 flex items-center justify-between">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground font-medium">{t("cards.kpi.active_cards")}</p>
              <p className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">{activeCount}</p>
            </div>
            <div className="w-10 h-10 rounded-xl bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 flex items-center justify-center">
              <Wifi className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>

        <Card className="bg-card/60 backdrop-blur-sm">
          <CardContent className="p-4 flex items-center justify-between">
            <div className="space-y-1">
              <p className="text-xs text-muted-foreground font-medium">{t("cards.kpi.locked_cards")}</p>
              <p className="text-2xl font-bold text-amber-600 dark:text-amber-400">{lockedCount}</p>
            </div>
            <div className="w-10 h-10 rounded-xl bg-amber-500/10 text-amber-600 dark:text-amber-400 flex items-center justify-center">
              <Lock className="w-5 h-5" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Filter & View Switcher Bar */}
      <div className="flex flex-col sm:flex-row items-center justify-between gap-3 bg-card p-3 rounded-xl border">
        <div className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
          <Input
            placeholder={t("cards.search_placeholder")}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-9 h-9 text-sm"
          />
        </div>

        <div className="flex items-center gap-2 self-end sm:self-auto">
          <div className="flex items-center bg-muted/70 p-0.5 rounded-lg border">
            <Button
              size="sm"
              variant={viewMode === "grid" ? "default" : "ghost"}
              className="h-7 px-2.5 text-xs gap-1.5 rounded-md"
              onClick={() => setViewMode("grid")}
            >
              <LayoutGrid className="w-3.5 h-3.5" />
              {t("cards.view_showcase")}
            </Button>
            <Button
              size="sm"
              variant={viewMode === "table" ? "default" : "ghost"}
              className="h-7 px-2.5 text-xs gap-1.5 rounded-md"
              onClick={() => setViewMode("table")}
            >
              <TableIcon className="w-3.5 h-3.5" />
              {t("cards.view_table")}
            </Button>
          </div>
        </div>
      </div>

      {/* Main Content Area */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 py-8">
          {[1, 2, 3].map((i) => (
            <div key={i} className="aspect-[1.586/1] rounded-2xl bg-muted/40 animate-pulse border" />
          ))}
        </div>
      ) : cards.length === 0 ? (
        /* Empty State */
        <div className="flex flex-col items-center justify-center py-16 text-center border rounded-2xl bg-card/40 p-8 space-y-4">
          <div className="w-16 h-16 rounded-3xl bg-primary/10 text-primary flex items-center justify-center shadow-inner">
            <Sparkles className="w-8 h-8" />
          </div>
          <div className="space-y-1 max-w-md">
            <h3 className="text-lg font-bold text-foreground">
              {t("cards.empty_title")}
            </h3>
            <p className="text-xs text-muted-foreground">
              {t("cards.empty_desc")}
            </p>
          </div>
          <div className="flex items-center gap-3 pt-2">
            <Button
              variant="outline"
              className="gap-2"
              onClick={() => setActivationOpen(true)}
              disabled={cardQuotaReached}
            >
              <KeyRound className="w-4 h-4 text-primary" />
              {t("cards.activate_card_btn")}
            </Button>
            <Button
              variant="default"
              className="gap-2"
              onClick={() => handleOpenSettings(null)}
              disabled={cardQuotaReached}
            >
              <Plus className="w-4 h-4" />
              {t("cards.create_card_btn")}
            </Button>
          </div>
        </div>
      ) : viewMode === "grid" ? (
        /* Vitrin Grid View */
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {cards.map((card) => (
            <InteractiveNFCCard
              key={card.uuid}
              card={card}
              onSettingsClick={() => handleOpenSettings(card)}
              onQRClick={() => handleOpenQR(card)}
              onToggleLock={() => handleToggleLock(card)}
            />
          ))}
        </div>
      ) : (
        /* Table View */
        <EntityTable
          data={cards}
          columns={tableColumns}
          isLoading={isLoading}
          manual={{ pagination: true }}
          pageCount={Math.ceil(total / pageSize)}
          state={{
            pagination: { pageIndex: page - 1, pageSize },
            onPaginationChange: (updater: Updater<PaginationState>) => {
              const next =
                typeof updater === "function"
                  ? updater({ pageIndex: page - 1, pageSize })
                  : updater;
              setPage(next.pageIndex + 1);
              setPageSize(next.pageSize);
            },
          }}
        />
      )}

      {/* Dialogs */}
      <CardActivationDialog
        open={activationOpen}
        onOpenChange={setActivationOpen}
        onSuccess={() => refetch()}
      />

      <CardSettingsDialog
        card={selectedCard}
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        onSuccess={() => refetch()}
      />

      <RouteQrDialog
        routeUuid={
          selectedCard && "route_uuid" in selectedCard
            ? selectedCard.route_uuid
            : undefined
        }
        title={selectedCard ? `${selectedCard.name} · /c/${selectedCard.smart_code}` : ""}
        open={qrOpen}
        onOpenChange={setQrOpen}
        suggestedLogoProfileUuid={selectedCard?.profile_uuid}
        fileStem={selectedCard ? `card-${selectedCard.smart_code}` : undefined}
      />

      <SubscriberOrderDialog
        open={orderOpen}
        onOpenChange={setOrderOpen}
        onSuccess={() => refetch()}
      />
    </div>
  );
}
