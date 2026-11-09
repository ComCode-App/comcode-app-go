"use client";

import { useCallback, useMemo, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ChevronDown,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCw,
  Search,
  Server,
  SlidersHorizontal,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Input } from "@/components/ui/input";
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { settingsNav } from "@/config/nav";
import { routes } from "@/config/routes";
import {
  useSystemInfo,
  useSystemPing,
} from "@/features/settings/hooks/use-system-info";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";

const STORAGE_KEY = "comcode_settings_subsidebar_collapsed";

export function SettingsSubSidebar({ className }: { className?: string }) {
  const { t } = useLocale();
  const pathname = usePathname();
  const { can } = usePermission();
  const [search, setSearch] = useState("");
  const [collapsed, setCollapsed] = useState(() => {
    if (typeof window !== "undefined") {
      try {
        const stored = localStorage.getItem(STORAGE_KEY);
        return stored === "true";
      } catch {
        // ignore
      }
    }
    return false;
  });
  const [openGroups, setOpenGroups] = useState<Record<string, boolean>>({
    system: true,
    general: true,
    security: true,
    billing: true,
    integrations: true,
    io: true,
  });

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev;
      try {
        localStorage.setItem(STORAGE_KEY, String(next));
      } catch {
        // ignore
      }
      return next;
    });
  };

  const toggleGroup = (groupId: string) => {
    setOpenGroups((prev) => ({
      ...prev,
      [groupId]: !prev[groupId],
    }));
  };

  const { data: systemInfo, isLoading: isSystemLoading } = useSystemInfo();
  const pingMutation = useSystemPing();

  const isHealthy = systemInfo?.status === "healthy";
  const isDegraded = systemInfo?.status === "degraded";

  const filteredGroups = useMemo(() => {
    const q = search.trim().toLowerCase();
    return settingsNav.groups
      .map((group) => {
        const visibleItems = group.items.filter((item) => {
          if (item.permission && !can(item.permission)) return false;
          if (!q) return true;
          const label = t(item.titleKey).toLowerCase();
          const groupLabel = t(group.labelKey).toLowerCase();
          return label.includes(q) || groupLabel.includes(q);
        });
        return {
          ...group,
          items: visibleItems,
        };
      })
      .filter((group) => group.items.length > 0);
  }, [search, can, t]);

  const checkIsActive = (href?: string) => {
    if (!href) return false;
    if (href === routes.platform.settings.root) {
      return (
        pathname === routes.platform.settings.root ||
        pathname === routes.platform.settings.system
      );
    }
    if (pathname === href) return true;
    if (href === routes.platform.settings.payment) {
      return pathname === routes.platform.settings.payment;
    }
    return pathname.startsWith(href + "/");
  };

  return (
    <aside
      className={cn(
        "border-border bg-card text-card-foreground hidden shrink-0 flex-col rounded-2xl border shadow-xs transition-[width] duration-200 ease-in-out lg:flex",
        collapsed ? "w-16" : "w-68",
        className,
      )}
    >
      {/* Top Header Card */}
      <div
        className={cn(
          "border-border border-b p-3",
          collapsed && "px-2 py-3 text-center",
        )}
      >
        <div className="flex items-center justify-between gap-1.5">
          {!collapsed && (
            <div className="flex min-w-0 items-center gap-2.5">
              <div className="bg-primary/10 text-primary border-primary/20 flex size-8 shrink-0 items-center justify-center rounded-lg border">
                <Server className="size-4" />
              </div>
              <div className="min-w-0">
                <h2 className="text-foreground truncate text-xs font-bold tracking-tight">
                  {t("settings.subnav.title")}
                </h2>
                <p className="text-muted-foreground truncate text-[10px]">
                  {t("settings.subnav.subtitle")}
                </p>
              </div>
            </div>
          )}

          <div
            className={cn(
              "flex items-center gap-1",
              collapsed && "mx-auto flex-col",
            )}
          >
            {!collapsed && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="text-muted-foreground hover:text-foreground size-7"
                    onClick={() => pingMutation.mutate()}
                    disabled={pingMutation.isPending || isSystemLoading}
                  >
                    <RefreshCw
                      className={cn("size-3.5", {
                        "animate-spin":
                          pingMutation.isPending || isSystemLoading,
                      })}
                    />
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="right">
                  {t("settings.system.refresh_tooltip")}
                </TooltipContent>
              </Tooltip>
            )}

            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground hover:text-foreground size-7"
                  onClick={toggleCollapsed}
                >
                  {collapsed ? (
                    <PanelLeftOpen className="text-primary size-4" />
                  ) : (
                    <PanelLeftClose className="size-4" />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right">
                {collapsed
                  ? t("layout.expand_sidebar")
                  : t("layout.collapse_sidebar")}
              </TooltipContent>
            </Tooltip>
          </div>
        </div>

        {/* Live System Indicator */}
        {!collapsed ? (
          <div className="border-border/80 bg-muted/40 mt-2.5 flex items-center justify-between rounded-lg border px-2.5 py-1.5 text-xs">
            <div className="flex min-w-0 items-center gap-2">
              <span className="relative flex size-2 shrink-0">
                <span
                  className={cn(
                    "absolute inline-flex h-full w-full animate-ping rounded-full opacity-75",
                    isHealthy
                      ? "bg-emerald-400"
                      : isDegraded
                        ? "bg-amber-400"
                        : "bg-rose-400",
                  )}
                />
                <span
                  className={cn(
                    "relative inline-flex size-2 rounded-full",
                    isHealthy
                      ? "bg-emerald-500"
                      : isDegraded
                        ? "bg-amber-500"
                        : "bg-rose-500",
                  )}
                />
              </span>
              <span className="text-foreground truncate text-[11px] font-medium">
                {isHealthy
                  ? t("settings.system.status_operational")
                  : isDegraded
                    ? t("settings.system.status_degraded")
                    : t("settings.system.status_down")}
              </span>
            </div>

            <Badge
              variant="outline"
              className="border-border h-4 px-1.5 py-0 font-mono text-[9px]"
            >
              {systemInfo?.runtime.go_version || "Go 1.24"}
            </Badge>
          </div>
        ) : (
          <div className="mt-2 flex justify-center">
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="bg-muted/60 flex size-6 items-center justify-center rounded-full">
                  <span className="relative flex size-2">
                    <span
                      className={cn(
                        "absolute inline-flex h-full w-full animate-ping rounded-full opacity-75",
                        isHealthy
                          ? "bg-emerald-400"
                          : isDegraded
                            ? "bg-amber-400"
                            : "bg-rose-400",
                      )}
                    />
                    <span
                      className={cn(
                        "relative inline-flex size-2 rounded-full",
                        isHealthy
                          ? "bg-emerald-500"
                          : isDegraded
                            ? "bg-amber-500"
                            : "bg-rose-500",
                      )}
                    />
                  </span>
                </div>
              </TooltipTrigger>
              <TooltipContent side="right">
                {isHealthy
                  ? t("settings.system.status_operational")
                  : isDegraded
                    ? t("settings.system.status_degraded")
                    : t("settings.system.status_down")}
              </TooltipContent>
            </Tooltip>
          </div>
        )}

        {/* Search Input when expanded */}
        {!collapsed && (
          <div className="relative mt-2.5">
            <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("settings.subnav.search_placeholder")}
              className="bg-background/70 border-border h-7.5 pl-8 text-xs"
            />
          </div>
        )}
      </div>

      {/* Nav List */}
      <ScrollArea className="min-h-0 flex-1 px-2 py-2 [&>[data-radix-scroll-area-viewport]]:overscroll-contain">
        <div className="space-y-3">
          {filteredGroups.map((group) => {
            const isGroupOpen =
              openGroups[group.id] !== false || search.length > 0;

            if (collapsed) {
              return (
                <div key={group.id} className="space-y-1">
                  {group.items.map((item) => {
                    const Icon = item.icon;
                    const isActive = checkIsActive(item.href);

                    return (
                      <Tooltip key={item.id}>
                        <TooltipTrigger asChild>
                          <Link
                            href={item.href ?? "#"}
                            className={cn(
                              "mx-auto flex size-9 items-center justify-center rounded-lg transition-colors",
                              isActive
                                ? "bg-primary text-primary-foreground font-semibold shadow-xs"
                                : "text-muted-foreground hover:bg-muted hover:text-foreground",
                            )}
                          >
                            <Icon className="size-4 shrink-0" />
                          </Link>
                        </TooltipTrigger>
                        <TooltipContent side="right" className="font-medium">
                          {t(item.titleKey)}
                        </TooltipContent>
                      </Tooltip>
                    );
                  })}
                </div>
              );
            }

            return (
              <Collapsible
                key={group.id}
                open={isGroupOpen}
                onOpenChange={() => toggleGroup(group.id)}
                className="space-y-1"
              >
                <CollapsibleTrigger asChild>
                  <button
                    type="button"
                    className="text-muted-foreground/80 hover:text-foreground group flex w-full items-center justify-between rounded-md px-2 py-1 text-[10px] font-bold tracking-wider uppercase transition-colors"
                  >
                    <span>{t(group.labelKey)}</span>
                    <ChevronDown
                      className={cn(
                        "text-muted-foreground/60 group-hover:text-foreground size-3 transition-transform duration-200",
                        isGroupOpen ? "rotate-0" : "-rotate-90",
                      )}
                    />
                  </button>
                </CollapsibleTrigger>

                <CollapsibleContent className="space-y-0.5 pt-0.5">
                  {group.items.map((item) => {
                    const Icon = item.icon;
                    const isActive = checkIsActive(item.href);

                    return (
                      <Link
                        key={item.id}
                        href={item.href ?? "#"}
                        className={cn(
                          "group flex items-center justify-between rounded-lg px-2.5 py-1.5 text-xs font-medium transition-all duration-150",
                          isActive
                            ? "bg-primary/10 text-primary ring-primary/20 dark:bg-primary/20 font-semibold ring-1"
                            : "text-muted-foreground hover:bg-muted hover:text-foreground",
                        )}
                      >
                        <div className="flex min-w-0 items-center gap-2">
                          <Icon
                            className={cn(
                              "size-3.5 shrink-0 transition-transform group-hover:scale-105",
                              isActive
                                ? "text-primary"
                                : "text-muted-foreground group-hover:text-foreground",
                            )}
                          />
                          <span className="truncate">{t(item.titleKey)}</span>
                        </div>
                        {isActive && (
                          <span className="bg-primary size-1.5 shrink-0 rounded-full" />
                        )}
                      </Link>
                    );
                  })}
                </CollapsibleContent>
              </Collapsible>
            );
          })}

          {!collapsed && filteredGroups.length === 0 && (
            <div className="text-muted-foreground py-6 text-center text-xs">
              {t("common.no_results")}
            </div>
          )}
        </div>
      </ScrollArea>

      {/* Footer Info */}
      <div
        className={cn(
          "border-border text-muted-foreground border-t p-2.5 text-[10px]",
          collapsed && "text-center",
        )}
      >
        {!collapsed ? (
          <div className="flex items-center justify-between">
            <span className="text-foreground/80 flex items-center gap-1.5 font-semibold">
              <span className="size-1.5 rounded-full bg-emerald-500" />
              {systemInfo?.environment.env
                ? systemInfo.environment.env.toUpperCase()
                : "PRODUCTION"}
            </span>
            <span className="text-muted-foreground font-mono">
              {systemInfo
                ? `${Math.floor(systemInfo.runtime.uptime_seconds / 3600)}h ${Math.floor((systemInfo.runtime.uptime_seconds % 3600) / 60)}m`
                : "0h 0m"}
            </span>
          </div>
        ) : (
          <Tooltip>
            <TooltipTrigger asChild>
              <div className="text-muted-foreground mx-auto flex size-6 items-center justify-center font-mono text-[9px]">
                {systemInfo
                  ? `${Math.floor(systemInfo.runtime.uptime_seconds / 3600)}h`
                  : "0h"}
              </div>
            </TooltipTrigger>
            <TooltipContent side="right">
              {systemInfo
                ? `${Math.floor(systemInfo.runtime.uptime_seconds / 3600)}h ${Math.floor((systemInfo.runtime.uptime_seconds % 3600) / 60)}m uptime`
                : "Uptime"}
            </TooltipContent>
          </Tooltip>
        )}
      </div>
    </aside>
  );
}

/**
 * Clean & modern mobile navigation bar + drawer for settings pages
 */
export function SettingsMobileNav({ className }: { className?: string }) {
  const { t } = useLocale();
  const pathname = usePathname();
  const { can } = usePermission();
  const [sheetOpen, setSheetOpen] = useState(false);
  const [search, setSearch] = useState("");

  const { data: systemInfo } = useSystemInfo();
  const isHealthy = systemInfo?.status === "healthy";
  const isDegraded = systemInfo?.status === "degraded";

  const checkIsActive = useCallback(
    (href?: string) => {
      if (!href) return false;
      if (href === routes.platform.settings.root) {
        return (
          pathname === routes.platform.settings.root ||
          pathname === routes.platform.settings.system
        );
      }
      if (pathname === href) return true;
      if (href === routes.platform.settings.payment) {
        return pathname === routes.platform.settings.payment;
      }
      return pathname.startsWith(href + "/");
    },
    [pathname],
  );

  let activeItem: {
    item?: (typeof settingsNav.groups)[0]["items"][0];
    group?: (typeof settingsNav.groups)[0];
  } = {
    item: settingsNav.groups[0]?.items[0],
    group: settingsNav.groups[0],
  };
  for (const group of settingsNav.groups) {
    for (const item of group.items) {
      if (checkIsActive(item.href)) {
        activeItem = { item, group };
        break;
      }
    }
  }

  const allVisibleItems = useMemo(() => {
    return settingsNav.groups.flatMap((g) =>
      g.items.filter((item) => !item.permission || can(item.permission)),
    );
  }, [can]);

  const filteredGroups = useMemo(() => {
    const q = search.trim().toLowerCase();
    return settingsNav.groups
      .map((group) => {
        const visibleItems = group.items.filter((item) => {
          if (item.permission && !can(item.permission)) return false;
          if (!q) return true;
          const label = t(item.titleKey).toLowerCase();
          const groupLabel = t(group.labelKey).toLowerCase();
          return label.includes(q) || groupLabel.includes(q);
        });
        return {
          ...group,
          items: visibleItems,
        };
      })
      .filter((group) => group.items.length > 0);
  }, [search, can, t]);

  const ActiveIcon = activeItem?.item?.icon ?? Server;

  return (
    <div
      className={cn(
        "border-border bg-card flex flex-col gap-2 rounded-xl border p-2.5 shadow-xs lg:hidden",
        className,
      )}
    >
      {/* Top row: Current active page info + Open Menu Sheet Trigger */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <div className="bg-primary/10 text-primary flex size-7 shrink-0 items-center justify-center rounded-md">
            <ActiveIcon className="size-3.5" />
          </div>
          <div className="min-w-0">
            <p className="text-muted-foreground truncate text-[10px] font-semibold tracking-wider uppercase">
              {activeItem?.group
                ? t(activeItem.group.labelKey)
                : t("settings.subnav.title")}
            </p>
            <h3 className="text-foreground truncate text-xs font-bold">
              {activeItem?.item ? t(activeItem.item.titleKey) : ""}
            </h3>
          </div>
        </div>

        <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
          <SheetTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              className="h-7.5 shrink-0 gap-1.5 px-2.5 text-xs font-medium"
            >
              <SlidersHorizontal className="text-primary size-3.5" />
              <span>{t("settings.subnav.title")}</span>
            </Button>
          </SheetTrigger>

          <SheetContent
            side="left"
            className="flex w-[85vw] max-w-sm flex-col p-0"
          >
            <SheetHeader className="border-border border-b p-4 text-left">
              <div className="flex items-center gap-2.5">
                <div className="bg-primary/10 text-primary border-primary/20 flex size-8 items-center justify-center rounded-lg border">
                  <Server className="size-4" />
                </div>
                <div>
                  <SheetTitle className="text-sm font-bold">
                    {t("settings.subnav.title")}
                  </SheetTitle>
                  <SheetDescription className="text-muted-foreground text-xs">
                    {t("settings.subnav.subtitle")}
                  </SheetDescription>
                </div>
              </div>

              {/* Live status badge inside sheet */}
              <div className="border-border/80 bg-muted/40 mt-3 flex items-center justify-between rounded-lg border px-2.5 py-1.5 text-xs">
                <div className="flex items-center gap-2">
                  <span className="relative flex size-2">
                    <span
                      className={cn(
                        "absolute inline-flex h-full w-full animate-ping rounded-full opacity-75",
                        isHealthy
                          ? "bg-emerald-400"
                          : isDegraded
                            ? "bg-amber-400"
                            : "bg-rose-400",
                      )}
                    />
                    <span
                      className={cn(
                        "relative inline-flex size-2 rounded-full",
                        isHealthy
                          ? "bg-emerald-500"
                          : isDegraded
                            ? "bg-amber-500"
                            : "bg-rose-500",
                      )}
                    />
                  </span>
                  <span className="text-foreground text-[11px] font-medium">
                    {isHealthy
                      ? t("settings.system.status_operational")
                      : isDegraded
                        ? t("settings.system.status_degraded")
                        : t("settings.system.status_down")}
                  </span>
                </div>
                <Badge
                  variant="outline"
                  className="border-border h-4 px-1.5 py-0 font-mono text-[9px]"
                >
                  {systemInfo?.runtime.go_version || "Go 1.24"}
                </Badge>
              </div>

              {/* Search input in sheet */}
              <div className="relative mt-2.5">
                <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2" />
                <Input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={t("settings.subnav.search_placeholder")}
                  className="bg-background h-8 pl-8 text-xs"
                />
              </div>
            </SheetHeader>

            {/* Nav list in Sheet */}
            <ScrollArea className="flex-1 px-3 py-3">
              <div className="space-y-4">
                {filteredGroups.map((group) => (
                  <div key={group.id} className="space-y-1">
                    <h4 className="text-muted-foreground/80 px-2 text-[10px] font-bold tracking-wider uppercase">
                      {t(group.labelKey)}
                    </h4>
                    <div className="space-y-0.5">
                      {group.items.map((item) => {
                        const Icon = item.icon;
                        const isActive = checkIsActive(item.href);

                        return (
                          <Link
                            key={item.id}
                            href={item.href ?? "#"}
                            onClick={() => setSheetOpen(false)}
                            className={cn(
                              "flex items-center justify-between rounded-lg px-2.5 py-2 text-xs font-medium transition-colors",
                              isActive
                                ? "bg-primary text-primary-foreground font-semibold shadow-xs"
                                : "text-muted-foreground hover:bg-muted hover:text-foreground",
                            )}
                          >
                            <div className="flex min-w-0 items-center gap-2.5">
                              <Icon className="size-4 shrink-0" />
                              <span className="truncate">
                                {t(item.titleKey)}
                              </span>
                            </div>
                            {isActive && (
                              <span className="bg-primary-foreground size-1.5 shrink-0 rounded-full" />
                            )}
                          </Link>
                        );
                      })}
                    </div>
                  </div>
                ))}

                {filteredGroups.length === 0 && (
                  <div className="text-muted-foreground py-6 text-center text-xs">
                    {t("common.no_results")}
                  </div>
                )}
              </div>
            </ScrollArea>
          </SheetContent>
        </Sheet>
      </div>

      {/* Horizontal Scrollable Pills Carousel for Quick Navigation on Mobile */}
      <div className="relative -mx-1">
        <ScrollArea className="w-full whitespace-nowrap">
          <div className="flex w-max space-x-1.5 px-1 py-0.5">
            {allVisibleItems.map((item) => {
              const Icon = item.icon;
              const isActive = checkIsActive(item.href);

              return (
                <Link
                  key={item.id}
                  href={item.href ?? "#"}
                  className={cn(
                    "inline-flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1 text-xs font-medium transition-colors",
                    isActive
                      ? "bg-primary text-primary-foreground font-semibold shadow-xs"
                      : "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground",
                  )}
                >
                  <Icon className="size-3.5 shrink-0" />
                  <span>{t(item.titleKey)}</span>
                </Link>
              );
            })}
          </div>
          <ScrollBar orientation="horizontal" className="invisible" />
        </ScrollArea>
      </div>
    </div>
  );
}
