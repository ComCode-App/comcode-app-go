"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Info } from "lucide-react";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import type { components } from "@/generated/api";
import { entitlementKeys } from "@/features/entitlements/hooks/use-entitlements";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { ProfileThemePicker } from "@/features/public-profile/components/theme-picker";
import { isLayoutMode, isThemeId, type LayoutMode, type ThemeId } from "@/features/public-profile/lib/themes";
import {
  PROFILE_PAGE_TARGETS,
  type SmartLink,
  type SmartLinkDetail,
  smartLinkKeys,
  type SmartLinkPresentation,
  type SmartLinkPrimaryAction,
  smartLinksService,
  type SmartLinkTarget,
} from "@/features/smart-links/services/smart-links.service";
import { isApiError, isPlanLimitError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

type PublicModule = components["schemas"]["PublicModule"];

const TARGETS: SmartLinkTarget[] = ["profile", "social_hub", "direct_url", "vcard_download"];
const MODULES: PublicModule[] = ["bio", "contacts", "social_links", "addresses", "bank_accounts"];
const PRIMARY_ACTIONS: SmartLinkPrimaryAction[] = ["save_contact", "call", "email", "whatsapp", "website"];
const PASSCODE_MIN = 4;

/** Stored presentation as the API returns it: the passcode itself is never sent back. */
type StoredPresentation = Omit<SmartLinkPresentation, "security"> & { security?: { passcode_enabled?: boolean } };

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Link to edit; undefined creates a new one. */
  link?: SmartLink;
};

export function SmartLinkDialog({ open, onOpenChange, link }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open ? <SmartLinkLoader key={link?.uuid ?? "new"} link={link} onDone={() => onOpenChange(false)} /> : null}
    </Dialog>
  );
}

/** The list carries no presentation_config, so editing loads the link first. */
function SmartLinkLoader({ link, onDone }: { link?: SmartLink; onDone: () => void }) {
  const detailQuery = useQuery({
    queryKey: smartLinkKeys.detail(link?.uuid ?? ""),
    queryFn: () => smartLinksService.get(link!.uuid),
    enabled: Boolean(link),
  });
  if (link && !detailQuery.data) {
    return (
      <DialogContent>
        <DialogHeader>
          <Skeleton className="h-6 w-40" />
        </DialogHeader>
        <Skeleton className="h-40 w-full" />
      </DialogContent>
    );
  }
  return <SmartLinkForm link={link} detail={detailQuery.data} onDone={onDone} />;
}

function SmartLinkForm({ link, detail, onDone }: { link?: SmartLink; detail?: SmartLinkDetail; onDone: () => void }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const profilesQuery = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
  });
  const stored = (detail?.presentation_config ?? {}) as StoredPresentation;
  const hadPasscode = stored.security?.passcode_enabled === true;

  const [slug, setSlug] = useState(link ? link.path.replace(/^s\//, "") : "");
  const [target, setTarget] = useState<SmartLinkTarget>((link?.target_type as SmartLinkTarget) || "profile");
  const [profileUuid, setProfileUuid] = useState(link?.profile_uuid ?? "");
  const [url, setUrl] = useState(link?.direct_url ?? "");
  const [theme, setTheme] = useState<ThemeId | "">(isThemeId(stored.theme) ? stored.theme : "");
  const [layoutMode, setLayoutMode] = useState<LayoutMode | "">(isLayoutMode(stored.layout_mode) ? stored.layout_mode : "");
  // null = the profile's own sections.
  const [modules, setModules] = useState<PublicModule[] | null>(stored.visible_modules ?? null);
  const [primaryAction, setPrimaryAction] = useState<SmartLinkPrimaryAction>(stored.primary_action?.type ?? "save_contact");
  const [leadCapture, setLeadCapture] = useState(stored.lead_capture?.enabled === true);
  const [passcodeOn, setPasscodeOn] = useState(hadPasscode);
  const [passcode, setPasscode] = useState("");
  const [error, setError] = useState<string | null>(null);

  const needsProfile = target !== "direct_url";
  const opensProfilePage = PROFILE_PAGE_TARGETS.includes(target);

  const presentation = (): SmartLinkPresentation => {
    const cfg: SmartLinkPresentation = {
      lead_capture: { enabled: leadCapture },
      security: passcodeOn
        ? { passcode_enabled: true, ...(passcode ? { passcode } : {}) }
        : { passcode_enabled: false },
    };
    if (opensProfilePage) {
      if (theme) cfg.theme = theme;
      if (layoutMode) cfg.layout_mode = layoutMode;
      if (modules && target === "profile") cfg.visible_modules = modules;
      cfg.primary_action = { type: primaryAction };
    }
    return cfg;
  };

  const save = useMutation({
    meta: { silentErrors: true },
    mutationFn: () => {
      const base = {
        target_type: target,
        profile_uuid: needsProfile ? profileUuid : undefined,
        direct_url: target === "direct_url" ? url.trim() : "",
        presentation_config: presentation(),
      };
      const path = `s/${slug.trim().toLowerCase().replace(/^s\//, "")}`;
      return link
        ? smartLinksService.update(link.uuid, { ...base, path })
        : smartLinksService.create({ ...base, slug: path });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["smart-links"] });
      void queryClient.invalidateQueries({ queryKey: entitlementKeys.overview });
      appToast.success(t(link ? "smart_links.toast.updated" : "smart_links.toast.created"));
      onDone();
    },
    onError: (err) => {
      if (isPlanLimitError(err)) {
        onDone();
        return; // the global plan-limit toast offers the upgrade
      }
      if (isApiError(err) && err.status === 409) setError(t("smart_links.error.slug_taken"));
      else if (isApiError(err) && /reserved/i.test(err.serverMessage ?? "")) setError(t("smart_links.error.slug_reserved"));
      else setError(isApiError(err) ? err.message : t("common.error_generic"));
    },
  });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    if (needsProfile && !profileUuid) return setError(t("smart_links.error.profile_required"));
    if (target === "direct_url" && !/^https:\/\/\S+$/i.test(url.trim())) return setError(t("smart_links.error.url_required"));
    if (passcodeOn && (passcode ? [...passcode].length < PASSCODE_MIN : !hadPasscode))
      return setError(t("smart_links.error.passcode_short", { min: PASSCODE_MIN }));
    save.mutate();
  };

  return (
    <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-2xl">
      <form onSubmit={submit} className="space-y-4">
        <DialogHeader>
          <DialogTitle>{t(link ? "smart_links.edit" : "smart_links.create")}</DialogTitle>
          <DialogDescription>{t("smart_links.subtitle")}</DialogDescription>
        </DialogHeader>

        <div className="grid gap-1.5">
          <Label htmlFor="smart-link-slug">{t("smart_links.slug")}</Label>
          <div className="flex items-center">
            <span className="text-muted-foreground bg-muted rounded-l-md border border-r-0 px-3 py-2 text-sm">/s/</span>
            <Input
              id="smart-link-slug"
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase())}
              className="rounded-l-none"
              autoComplete="off"
              spellCheck={false}
              maxLength={100}
              required
            />
          </div>
          <p className="text-muted-foreground text-xs">{t("smart_links.slug_hint")}</p>
        </div>

        <div className="grid gap-1.5">
          <Label>{t("smart_links.target")}</Label>
          <Select value={target} onValueChange={(v) => setTarget(v as SmartLinkTarget)}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TARGETS.map((tt) => (
                <SelectItem key={tt} value={tt}>
                  {t(`smart_links.target.${tt}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {target === "social_hub" ? (
            <p className="text-muted-foreground text-xs">{t("smart_links.target_hint.social_hub")}</p>
          ) : null}
        </div>

        {needsProfile ? (
          <div className="grid gap-1.5">
            <Label>{t("smart_links.profile")}</Label>
            <Select value={profileUuid} onValueChange={setProfileUuid}>
              <SelectTrigger>
                <SelectValue placeholder={t("smart_links.profile_placeholder")} />
              </SelectTrigger>
              <SelectContent>
                {(profilesQuery.data?.items ?? []).map((p) => (
                  <SelectItem key={p.uuid} value={p.uuid}>
                    {`${p.first_name} ${p.last_name}`.trim()}
                    {p.username ? ` · /${p.username}` : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        ) : (
          <div className="grid gap-1.5">
            <Label htmlFor="smart-link-url">{t("smart_links.url")}</Label>
            <Input
              id="smart-link-url"
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder={t("smart_links.url_placeholder")}
            />
          </div>
        )}

        <Separator />
        <div>
          <p className="text-sm font-medium">{t("smart_links.presentation.title")}</p>
          <p className="text-muted-foreground text-xs">{t("smart_links.presentation.subtitle")}</p>
        </div>

        {opensProfilePage ? (
          <>
            <ProfileThemePicker
              theme={theme}
              layoutMode={layoutMode}
              inheritedTheme=""
              disabled={save.isPending}
              noneLabel={t("smart_links.presentation.theme_profile")}
              hint={t("smart_links.presentation.theme_hint")}
              onThemeChange={setTheme}
              onLayoutModeChange={setLayoutMode}
            />

            {target === "profile" ? (
              <div className="space-y-2">
                <label className="flex items-center justify-between gap-3 text-sm font-medium">
                  {t("smart_links.presentation.custom_modules")}
                  <Switch
                    checked={modules !== null}
                    onCheckedChange={(on) => setModules(on ? [...MODULES] : null)}
                    disabled={save.isPending}
                  />
                </label>
                <p className="text-muted-foreground text-xs">{t("smart_links.presentation.modules_hint")}</p>
                {modules !== null ? (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {MODULES.map((m) => (
                      <label key={m} className="flex items-center gap-2 text-sm">
                        <Checkbox
                          checked={modules.includes(m)}
                          onCheckedChange={(on) =>
                            setModules((list) =>
                              on === true ? MODULES.filter((x) => x === m || list?.includes(x)) : (list ?? []).filter((x) => x !== m),
                            )
                          }
                          disabled={save.isPending}
                        />
                        {t(`profiles.public_page.module.${m}`)}
                      </label>
                    ))}
                  </div>
                ) : null}
              </div>
            ) : null}

            <div className="grid gap-1.5">
              <Label>{t("smart_links.presentation.primary_action")}</Label>
              <Select value={primaryAction} onValueChange={(v) => setPrimaryAction(v as SmartLinkPrimaryAction)}>
                <SelectTrigger className="w-full sm:w-72">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PRIMARY_ACTIONS.map((a) => (
                    <SelectItem key={a} value={a}>
                      {t(`smart_links.primary_action.${a}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-muted-foreground text-xs">{t("smart_links.presentation.primary_action_hint")}</p>
            </div>
          </>
        ) : null}

        <div className="space-y-1.5">
          <label className="flex items-center justify-between gap-3 text-sm font-medium">
            {t("smart_links.presentation.lead_capture")}
            <Switch checked={leadCapture} onCheckedChange={setLeadCapture} disabled={save.isPending} />
          </label>
          {opensProfilePage ? (
            <p className="text-muted-foreground text-xs">{t("smart_links.presentation.lead_capture_hint")}</p>
          ) : (
            <p className="flex items-start gap-1.5 text-xs text-amber-600 dark:text-amber-400">
              <Info className="mt-px size-3.5 shrink-0" />
              {t("smart_links.presentation.lead_capture_needs_profile")}
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <label className="flex items-center justify-between gap-3 text-sm font-medium">
            {t("smart_links.presentation.passcode")}
            <Switch checked={passcodeOn} onCheckedChange={setPasscodeOn} disabled={save.isPending} />
          </label>
          <p className="text-muted-foreground text-xs">{t("smart_links.presentation.passcode_hint")}</p>
          {passcodeOn ? (
            <Input
              type="password"
              value={passcode}
              onChange={(e) => setPasscode(e.target.value)}
              autoComplete="new-password"
              maxLength={64}
              aria-label={t("smart_links.presentation.passcode")}
              placeholder={
                hadPasscode
                  ? t("smart_links.presentation.passcode_keep")
                  : t("smart_links.presentation.passcode_placeholder", { min: PASSCODE_MIN })
              }
              className="sm:w-72"
            />
          ) : null}
        </div>

        {error ? <p className="text-destructive text-sm">{error}</p> : null}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onDone}>
            {t("smart_links.cancel")}
          </Button>
          <Button type="submit" disabled={save.isPending || !slug.trim()}>
            {t("smart_links.save")}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  );
}
