"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, ExternalLink, Globe, QrCode } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { backgroundKeys, backgroundsService } from "@/features/background-studio/services/backgrounds.service";
import { useBankAccounts } from "@/features/bank-accounts";
import {
  PremiumLockIcon,
  PremiumNotice,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { ProfileThemePicker } from "@/features/public-profile/components/theme-picker";
import type { LayoutMode, ThemeId } from "@/features/public-profile/lib/themes";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  type PublicModule,
  type PublicPageSettings,
  profilesService,
} from "@/features/profiles/services/profiles.service";
import { RouteQrDialog, routeQrKeys, routeQrService } from "@/features/route-qr";
import { isApiError } from "@/lib/api";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

const MODULES: PublicModule[] = ["bio", "contacts", "social_links", "addresses", "bank_accounts"];

/** Owner settings for comcode.../{username}: address, on/off, sections and published banks. */
export function ProfilePublicPageCard({ profileUuid }: { profileUuid: string }) {
  const pageQuery = useQuery({
    queryKey: profilesKeys.publicPage(profileUuid),
    queryFn: () => profilesService.getPublicPage(profileUuid),
  });
  const page = pageQuery.data;
  if (!page) return null;
  // Remount on every saved state so the form starts from the server's values.
  return <PublicPageForm key={JSON.stringify(page)} profileUuid={profileUuid} page={page} />;
}

function PublicPageForm({ profileUuid, page }: { profileUuid: string; page: PublicPageSettings }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const canEdit = canManageAccount(user);
  const queryClient = useQueryClient();
  const banksQuery = useBankAccounts();
  const backgroundsQuery = useQuery({ queryKey: backgroundKeys.list, queryFn: backgroundsService.list });

  const [username, setUsername] = useState(page.username);
  const [enabled, setEnabled] = useState(page.enabled);
  const [modules, setModules] = useState<PublicModule[]>(page.visible_modules);
  const [banks, setBanks] = useState<string[]>(page.bank_account_uuids);
  const [usernameError, setUsernameError] = useState<string | null>(null);
  const [background, setBackground] = useState<string | null>(page.background_uuid ?? null);
  const [leadCapture, setLeadCapture] = useState(page.lead_capture_enabled ?? false);
  const [exchange, setExchange] = useState(page.contact_exchange_enabled ?? false);
  const [theme, setTheme] = useState<ThemeId | "">(page.theme ?? "");
  const [layoutMode, setLayoutMode] = useState<LayoutMode | "">(page.layout_mode ?? "");
  const [qrOpen, setQrOpen] = useState(false);
  // Animated backgrounds need design.premium (TEC-947); a stored one stays selectable.
  const allowPremium = useDesignPremium();
  const backgrounds = backgroundsQuery.data ?? [];
  const selectedBackground = backgrounds.find((b) => b.uuid === background);
  const hasLockedBackground = !allowPremium && backgrounds.some((b) => b.premium);
  const canReadRoutes = usePermission().can(permissions.routes.appRead);
  // The QR of a public page encodes its canonical /{username} route identity.
  const canonicalRoute = useQuery({
    queryKey: routeQrKeys.canonical(profileUuid, page.username),
    queryFn: () => routeQrService.canonicalRoute(profileUuid, page.username),
    enabled: canReadRoutes && Boolean(page.username),
  });

  const save = useMutation({
    mutationFn: () =>
      profilesService.updatePublicPage(profileUuid, {
        username: username.trim().toLowerCase(),
        enabled,
        visible_modules: modules,
        bank_account_uuids: banks,
        background_uuid: background,
        lead_capture_enabled: leadCapture,
        contact_exchange_enabled: exchange,
        theme,
        layout_mode: layoutMode,
      }),
    onSuccess: (page) => {
      setUsernameError(null);
      queryClient.setQueryData(profilesKeys.publicPage(profileUuid), page);
      void queryClient.invalidateQueries({ queryKey: profilesKeys.lists() });
      void queryClient.invalidateQueries({ queryKey: profilesKeys.detail(profileUuid) });
      appToast.success(t("profiles.public_page.saved"));
    },
    meta: { silentErrors: true },
    onError: (err) => {
      if (isApiError(err) && err.status === 409) setUsernameError(t("profiles.public_page.username_taken"));
      else if (isApiError(err) && err.status === 400) setUsernameError(err.message);
      else appToast.error(isApiError(err) ? err.message : t("common.error_generic"));
    },
  });

  const publicUrl = typeof window === "undefined" ? `/${page.username}` : `${window.location.origin}/${page.username}`;
  const toggle = <T,>(list: T[], value: T, on: boolean) =>
    on ? Array.from(new Set([...list, value])) : list.filter((v) => v !== value);
  const dirty =
    username.trim().toLowerCase() !== page.username ||
    enabled !== page.enabled ||
    modules.join() !== page.visible_modules.join() ||
    [...banks].sort().join() !== [...page.bank_account_uuids].sort().join() ||
    background !== (page.background_uuid ?? null) ||
    leadCapture !== (page.lead_capture_enabled ?? false) ||
    exchange !== (page.contact_exchange_enabled ?? false) ||
    theme !== (page.theme ?? "") ||
    layoutMode !== (page.layout_mode ?? "");

  return (
    <Card className="border-border/80 shadow-sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Globe className="text-primary size-4" />
          {t("profiles.public_page.title")}
        </CardTitle>
        <CardDescription>{t("profiles.public_page.subtitle")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="bg-muted/50 flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center">
          <code className="min-w-0 flex-1 truncate text-sm">{publicUrl}</code>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() =>
                void navigator.clipboard
                  .writeText(publicUrl)
                  .then(() => appToast.success(t("profiles.public_page.link_copied")))
              }
            >
              <Copy className="size-4" />
              {t("profiles.public_page.copy_link")}
            </Button>
            {canonicalRoute.data ? (
              <Button type="button" variant="outline" size="sm" onClick={() => setQrOpen(true)}>
                <QrCode className="size-4" />
                {t("routes.qr.open")}
              </Button>
            ) : null}
            <Button asChild variant="outline" size="sm" disabled={!page.enabled}>
              <a href={`/${page.username}`} target="_blank" rel="noopener noreferrer">
                <ExternalLink className="size-4" />
                {t("profiles.public_page.open")}
              </a>
            </Button>
          </div>
        </div>

        <RouteQrDialog
          routeUuid={canonicalRoute.data?.uuid}
          title={`/${page.username}`}
          open={qrOpen}
          onOpenChange={setQrOpen}
          suggestedLogoProfileUuid={profileUuid}
          fileStem={page.username}
        />

        {!canEdit ? <p className="text-muted-foreground text-sm">{t("profiles.public_page.read_only")}</p> : null}

        <fieldset disabled={!canEdit || save.isPending} className="space-y-6">
          <div className="flex items-start justify-between gap-4">
            <div>
              <Label htmlFor="public-page-enabled">{t("profiles.public_page.enabled")}</Label>
              <p className="text-muted-foreground mt-1 text-xs">{t("profiles.public_page.disabled_hint")}</p>
            </div>
            <Switch
              id="public-page-enabled"
              checked={enabled}
              onCheckedChange={setEnabled}
              disabled={!canEdit || save.isPending}
            />
          </div>

          <div className="flex items-start justify-between gap-4">
            <div>
              <Label htmlFor="public-page-lead-capture">{t("profiles.public_page.lead_capture")}</Label>
              <p className="text-muted-foreground mt-1 text-xs">{t("profiles.public_page.lead_capture_hint")}</p>
            </div>
            <Switch
              id="public-page-lead-capture"
              checked={leadCapture}
              onCheckedChange={setLeadCapture}
              disabled={!canEdit || save.isPending}
            />
          </div>

          <div className="flex items-start justify-between gap-4">
            <div>
              <Label htmlFor="public-page-exchange">{t("profiles.public_page.contact_exchange")}</Label>
              <p className="text-muted-foreground mt-1 text-xs">{t("profiles.public_page.contact_exchange_hint")}</p>
            </div>
            <Switch
              id="public-page-exchange"
              checked={exchange}
              onCheckedChange={setExchange}
              disabled={!canEdit || save.isPending}
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="public-page-username">{t("profiles.public_page.username")}</Label>
            <div className="flex items-center">
              <span className="text-muted-foreground bg-muted rounded-l-md border border-r-0 px-3 py-2 text-sm">/</span>
              <Input
                id="public-page-username"
                value={username}
                onChange={(e) => {
                  setUsername(e.target.value.toLowerCase());
                  setUsernameError(null);
                }}
                autoComplete="off"
                spellCheck={false}
                maxLength={40}
                className="rounded-l-none"
                aria-invalid={Boolean(usernameError)}
              />
            </div>
            <p className={usernameError ? "text-destructive text-xs" : "text-muted-foreground text-xs"}>
              {usernameError ?? t("profiles.public_page.username_hint")}
            </p>
          </div>

          <ProfileThemePicker
            theme={theme}
            layoutMode={layoutMode}
            inheritedTheme={page.inherited_theme ?? ""}
            disabled={!canEdit || save.isPending}
            onThemeChange={setTheme}
            onLayoutModeChange={setLayoutMode}
          />
          {page.branding_locked ? (
            // TEC-946: the company look overrides this profile's theme, layout and cover.
            <p className="text-muted-foreground -mt-2 text-xs">{t("profiles.public_page.branding_locked")}</p>
          ) : null}

          <div className="grid gap-1.5">
            <Label>{t("profiles.public_page.background")}</Label>
            <Select
              value={background ?? "default"}
              onValueChange={(value) => setBackground(value === "default" ? null : value)}
              disabled={!canEdit || save.isPending}
            >
              <SelectTrigger className="w-full sm:w-80">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="default">{t("profiles.public_page.background_default")}</SelectItem>
                {backgrounds.map((b) => {
                  const locked = b.premium && !allowPremium;
                  return (
                    <SelectItem key={b.uuid} value={b.uuid} disabled={locked && b.uuid !== page.background_uuid}>
                      {b.name}
                      {b.is_preset ? ` · ${t("background_studio.saved.preset")}` : ""}
                      {locked ? <PremiumLockIcon /> : null}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
            <p className="text-muted-foreground text-xs">
              {t("profiles.public_page.background_hint")}{" "}
              <Link href={routes.app.backgroundStudio.root} className="text-primary hover:underline">
                {t("profiles.public_page.background_studio_link")}
              </Link>
            </p>
            {selectedBackground?.premium && !allowPremium ? (
              <p className="text-xs text-amber-700 dark:text-amber-400">
                {t("profiles.public_page.background_premium_fallback")}
              </p>
            ) : null}
            {hasLockedBackground ? <PremiumNotice /> : null}
          </div>

          <div className="space-y-2">
            <p className="text-sm font-medium">{t("profiles.public_page.modules")}</p>
            <div className="grid gap-2 sm:grid-cols-2">
              {MODULES.map((m) => (
                <label key={m} className="flex items-center gap-2 text-sm">
                  <Checkbox
                    checked={modules.includes(m)}
                    onCheckedChange={(on) => setModules((list) => toggle(list, m, on === true))}
                    disabled={!canEdit || save.isPending}
                  />
                  {t(`profiles.public_page.module.${m}`)}
                </label>
              ))}
            </div>
          </div>

          {modules.includes("bank_accounts") ? (
            <div className="space-y-2">
              <p className="text-sm font-medium">{t("profiles.public_page.banks")}</p>
              <p className="text-muted-foreground text-xs">{t("profiles.public_page.banks_hint")}</p>
              {banksQuery.data && banksQuery.data.length === 0 ? (
                <p className="text-muted-foreground text-sm">{t("profiles.public_page.banks_empty")}</p>
              ) : null}
              <div className="space-y-2">
                {(banksQuery.data ?? []).map((b) => (
                  <label key={b.uuid} className="flex items-center gap-3 rounded-md border p-2 text-sm">
                    <Checkbox
                      checked={banks.includes(b.uuid)}
                      onCheckedChange={(on) => setBanks((list) => toggle(list, b.uuid, on === true))}
                      disabled={!canEdit || save.isPending}
                    />
                    <span className="min-w-0">
                      <span className="block truncate font-medium">{b.bank_name || b.title || b.account_holder}</span>
                      <span className="text-muted-foreground block truncate font-mono text-xs">
                        {b.formatted_iban || b.iban}
                      </span>
                    </span>
                  </label>
                ))}
              </div>
            </div>
          ) : null}

          {canEdit ? (
            <div className="flex justify-end">
              <Button type="button" onClick={() => save.mutate()} disabled={!dirty || save.isPending}>
                {t("profiles.public_page.save")}
              </Button>
            </div>
          ) : null}
        </fieldset>
      </CardContent>
    </Card>
  );
}
