"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ExternalLink, FileText, Trash2, Upload } from "lucide-react";
import { type FormEvent, useRef, useState } from "react";

import { Loading } from "@/components/common/loading";
import { EntityPage, EntitySectionCard, EntityUnavailable } from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
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
import {
  PremiumLockIcon,
  PremiumNotice,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { LAYOUT_MODES, THEME_IDS, isPremiumTheme } from "@/features/public-profile/lib/themes";
import { ApiError } from "@/lib/api/errors";
import { assertServiceMediaURL } from "@/lib/media/urls";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { useCompanyBrandingAccess } from "../lib/access";
import {
  type CompanyBranding,
  type CompanyMedia,
  type CompanyMediaKind,
  type CompanyMediaUsage,
  MEDIA_KINDS,
  companyBrandingKeys,
  companyBrandingService,
} from "../services/company-branding.service";

const NONE = "__none__";
const IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"];
const MAX_IMAGE_BYTES = 5 << 20;
const MAX_DOCUMENT_BYTES = 10 << 20;
const COLOR_RE = /^#[0-9a-f]{6}$/;

type Translate = ReturnType<typeof useLocale>["t"];

function errorMessage(err: unknown, t: Translate) {
  if (err instanceof ApiError) {
    if (err.status === 403) return t("company_branding.errors.forbidden");
    if (err.status === 409 && err.details.length) return t("company_branding.errors.in_use");
  }
  return err instanceof Error && err.message ? err.message : t("common.error_generic");
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function usageLabel(u: CompanyMediaUsage, t: Translate) {
  const label = t(`company_branding.usage.${u.type}`);
  return u.profile_name ? `${label}: ${u.profile_name}` : label;
}

/**
 * Company media library and branding (TEC-946). The branding is the look employee public pages
 * inherit (TEC-601): their default while unlocked, forced while locked. Logo and cover are
 * picked from the library.
 */
export function CompanyBrandingPage() {
  const { t } = useLocale();
  const access = useCompanyBrandingAccess();

  return (
    <EntityPage title={t("company_branding.title")} description={t("company_branding.description")}>
      {!access.isCompany ? (
        <EntityUnavailable
          title={t("company_branding.unavailable.personal_title")}
          description={t("company_branding.unavailable.personal_description")}
        />
      ) : !access.canRead ? (
        <EntityUnavailable title={t("common.error_forbidden")} description={t("company_branding.errors.forbidden")} />
      ) : (
        <div className="space-y-6">
          <BrandingCard canWrite={access.canWrite} />
          <MediaLibraryCard canWrite={access.canWrite} />
        </div>
      )}
    </EntityPage>
  );
}

function useLibrary() {
  return useQuery({
    queryKey: companyBrandingKeys.media("all"),
    queryFn: () => companyBrandingService.listMedia(),
  });
}

type BrandingForm = {
  theme: string;
  layout: string;
  color: string;
  logo: string;
  cover: string;
  locked: boolean;
};

function toForm(b: CompanyBranding): BrandingForm {
  return {
    theme: b.theme || NONE,
    layout: b.layout_mode || NONE,
    color: b.primary_color,
    logo: b.logo?.uuid ?? NONE,
    cover: b.cover?.uuid ?? NONE,
    locked: b.locked,
  };
}

function BrandingCard({ canWrite }: { canWrite: boolean }) {
  const { t } = useLocale();
  const brandingQuery = useQuery({
    queryKey: companyBrandingKeys.branding,
    queryFn: companyBrandingService.getBranding,
  });

  if (!brandingQuery.data) {
    return (
      <EntitySectionCard title={t("company_branding.branding.title")}>
        <Loading />
      </EntitySectionCard>
    );
  }
  // Remount on every saved version so the form starts from the stored branding.
  return (
    <BrandingFormCard
      key={brandingQuery.data.updated_at ?? "new"}
      branding={brandingQuery.data}
      canWrite={canWrite}
    />
  );
}

function BrandingFormCard({ branding, canWrite }: { branding: CompanyBranding; canWrite: boolean }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const library = useLibrary();
  const [form, setForm] = useState<BrandingForm>(() => toForm(branding));

  const save = useMutation({
    mutationFn: companyBrandingService.updateBranding,
    onSuccess: (data) => {
      queryClient.setQueryData(companyBrandingKeys.branding, data);
      void queryClient.invalidateQueries({ queryKey: companyBrandingKeys.all });
      appToast.success(t("company_branding.toast.saved"));
    },
    onError: (err) => appToast.error(errorMessage(err, t)),
  });

  const images = (library.data?.items ?? []).filter((m) => m.kind !== "document");
  const set = (patch: Partial<BrandingForm>) => setForm({ ...form, ...patch });
  const color = form.color.trim().toLowerCase();
  const colorInvalid = color !== "" && !COLOR_RE.test(color);
  const disabled = !canWrite || save.isPending;
  // Premium themes need design.premium (TEC-947); the stored one stays selectable.
  const allowPremium = useDesignPremium();
  const pick = (value: string) => (value === NONE ? null : value);

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (colorInvalid) return;
    save.mutate({
      theme: pick(form.theme) ?? "",
      layout_mode: pick(form.layout) ?? "",
      primary_color: color,
      logo_media_uuid: pick(form.logo),
      cover_media_uuid: pick(form.cover),
      locked: form.locked,
    });
  };

  const imageSelect = (id: string, label: string, value: string, onChange: (v: string) => void) => {
    const selected = images.find((m) => m.uuid === value);
    return (
      <div className="grid gap-1.5">
        <Label htmlFor={id}>{label}</Label>
        <div className="flex items-center gap-3">
          {selected ? (
            // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, any host
            <img src={assertServiceMediaURL(selected.url) ?? undefined} alt="" className="size-10 rounded border object-contain" />
          ) : null}
          <Select value={value} onValueChange={onChange} disabled={disabled}>
            <SelectTrigger id={id} className="w-full sm:w-72">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>{t("company_branding.branding.none")}</SelectItem>
              {images.map((m) => (
                <SelectItem key={m.uuid} value={m.uuid}>
                  {m.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    );
  };

  return (
    <EntitySectionCard title={t("company_branding.branding.title")}>
      <form onSubmit={onSubmit} className="space-y-5">
        <p className="text-muted-foreground text-sm">{t("company_branding.branding.hint")}</p>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="grid gap-1.5">
            <Label htmlFor="branding-theme">{t("company_branding.branding.theme")}</Label>
            <Select value={form.theme} onValueChange={(v) => set({ theme: v })} disabled={disabled}>
              <SelectTrigger id="branding-theme" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NONE}>{t("company_branding.branding.theme_none")}</SelectItem>
                {THEME_IDS.map((id) => {
                  const locked = !allowPremium && isPremiumTheme(id);
                  return (
                    <SelectItem key={id} value={id} disabled={locked && id !== branding.theme}>
                      {t(`profiles.public_page.theme_name.${id}`)}
                      {locked ? <PremiumLockIcon /> : null}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
            {!allowPremium ? <PremiumNotice /> : null}
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="branding-layout">{t("company_branding.branding.layout")}</Label>
            <Select value={form.layout} onValueChange={(v) => set({ layout: v })} disabled={disabled}>
              <SelectTrigger id="branding-layout" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NONE}>{t("company_branding.branding.layout_theme")}</SelectItem>
                {LAYOUT_MODES.map((id) => (
                  <SelectItem key={id} value={id}>
                    {t(`profiles.public_page.layout_name.${id}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="grid gap-1.5">
          <Label htmlFor="branding-color">{t("company_branding.branding.primary_color")}</Label>
          <div className="flex items-center gap-2">
            <input
              type="color"
              aria-label={t("company_branding.branding.primary_color")}
              value={COLOR_RE.test(color) ? color : "#000000"}
              onChange={(e) => set({ color: e.target.value })}
              disabled={disabled}
              className="h-9 w-12 cursor-pointer rounded border bg-transparent p-1"
            />
            <Input
              id="branding-color"
              value={form.color}
              onChange={(e) => set({ color: e.target.value })}
              placeholder="#1a73e8"
              maxLength={7}
              className="w-32 font-mono"
              aria-invalid={colorInvalid}
              disabled={disabled}
            />
            {form.color ? (
              <Button type="button" variant="ghost" size="sm" onClick={() => set({ color: "" })} disabled={disabled}>
                {t("company_branding.branding.clear")}
              </Button>
            ) : null}
          </div>
          <p className={colorInvalid ? "text-destructive text-xs" : "text-muted-foreground text-xs"}>
            {colorInvalid ? t("company_branding.branding.color_invalid") : t("company_branding.branding.color_hint")}
          </p>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {imageSelect("branding-logo", t("company_branding.branding.logo"), form.logo, (v) => set({ logo: v }))}
          {imageSelect("branding-cover", t("company_branding.branding.cover"), form.cover, (v) => set({ cover: v }))}
        </div>
        {!images.length ? (
          <p className="text-muted-foreground text-xs">{t("company_branding.branding.no_images")}</p>
        ) : null}

        <div className="flex items-start justify-between gap-4 rounded-md border p-3">
          <div>
            <Label htmlFor="branding-locked">{t("company_branding.branding.locked")}</Label>
            <p className="text-muted-foreground mt-1 text-xs">
              {form.locked ? t("company_branding.branding.locked_on") : t("company_branding.branding.locked_off")}
            </p>
          </div>
          <Switch
            id="branding-locked"
            checked={form.locked}
            onCheckedChange={(v) => set({ locked: v })}
            disabled={disabled}
          />
        </div>

        {canWrite ? (
          <Button type="submit" disabled={save.isPending || colorInvalid}>
            {t("company_branding.branding.save")}
          </Button>
        ) : (
          <p className="text-muted-foreground text-xs">{t("company_branding.read_only")}</p>
        )}
      </form>
    </EntitySectionCard>
  );
}

function MediaLibraryCard({ canWrite }: { canWrite: boolean }) {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const queryClient = useQueryClient();
  const library = useLibrary();
  const fileRef = useRef<HTMLInputElement>(null);
  const [kind, setKind] = useState<CompanyMediaKind>("logo");
  const [name, setName] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const invalidate = () => void queryClient.invalidateQueries({ queryKey: companyBrandingKeys.all });

  const upload = useMutation({
    mutationFn: companyBrandingService.uploadMedia,
    onSuccess: () => {
      setFile(null);
      setName("");
      if (fileRef.current) fileRef.current.value = "";
      invalidate();
      appToast.success(t("company_branding.toast.uploaded"));
    },
    onError: (err) => appToast.error(errorMessage(err, t)),
  });
  const remove = useMutation({
    mutationFn: companyBrandingService.deleteMedia,
    onSuccess: () => {
      invalidate();
      appToast.success(t("company_branding.toast.deleted"));
    },
    onError: (err) => appToast.error(errorMessage(err, t)),
  });

  const fileError = (() => {
    if (!file) return null;
    if (kind === "document") {
      if (file.type !== "application/pdf") return t("company_branding.library.invalid_document");
      if (file.size > MAX_DOCUMENT_BYTES) return t("company_branding.library.document_too_large");
      return null;
    }
    if (!IMAGE_TYPES.includes(file.type)) return t("company_branding.library.invalid_image");
    if (file.size > MAX_IMAGE_BYTES) return t("company_branding.library.image_too_large");
    return null;
  })();

  const onSubmit = (event: FormEvent) => {
    event.preventDefault();
    if (!file || fileError) return;
    upload.mutate({ file, kind, name: name.trim() || undefined });
  };

  const confirmRemove = async (item: CompanyMedia) => {
    const confirmed = await confirmDelete({
      title: t("company_branding.library.delete_title"),
      description: t("company_branding.library.delete_description", { name: item.name }),
    });
    if (confirmed) remove.mutate(item.uuid);
  };

  const items = library.data?.items ?? [];

  return (
    <EntitySectionCard title={t("company_branding.library.title")}>
      <div className="space-y-5">
        {canWrite ? (
          <form onSubmit={onSubmit} className="grid gap-3 sm:grid-cols-[10rem_1fr_1fr_auto] sm:items-end">
            <div className="grid gap-1.5">
              <Label htmlFor="media-kind">{t("company_branding.library.kind")}</Label>
              <Select value={kind} onValueChange={(v) => setKind(v as CompanyMediaKind)}>
                <SelectTrigger id="media-kind" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {MEDIA_KINDS.map((k) => (
                    <SelectItem key={k} value={k}>
                      {t(`company_branding.kind.${k}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="media-file">{t("company_branding.library.file")}</Label>
              <Input
                id="media-file"
                ref={fileRef}
                type="file"
                accept={kind === "document" ? "application/pdf" : IMAGE_TYPES.join(",")}
                onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="media-name">{t("company_branding.library.name")}</Label>
              <Input
                id="media-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={255}
                placeholder={file?.name ?? ""}
              />
            </div>
            <Button type="submit" disabled={!file || Boolean(fileError) || upload.isPending}>
              <Upload className="size-4" />
              {t("company_branding.library.upload")}
            </Button>
            <p className={fileError ? "text-destructive text-xs sm:col-span-4" : "text-muted-foreground text-xs sm:col-span-4"}>
              {fileError ?? t("company_branding.library.upload_hint")}
            </p>
          </form>
        ) : null}

        {library.isLoading ? (
          <Loading />
        ) : !items.length ? (
          <p className="text-muted-foreground text-sm">{t("company_branding.library.empty")}</p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((item) => {
              const url = assertServiceMediaURL(item.url) ?? undefined;
              return (
                <li key={item.uuid} className="flex gap-3 rounded-md border p-3">
                  <div className="bg-muted flex size-16 shrink-0 items-center justify-center overflow-hidden rounded">
                    {item.kind === "document" ? (
                      <FileText className="text-muted-foreground size-6" />
                    ) : (
                      // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, any host
                      <img src={url} alt="" className="h-full w-full object-contain" loading="lazy" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1 space-y-1">
                    <p className="truncate text-sm font-medium" title={item.name}>
                      {item.name}
                    </p>
                    <div className="text-muted-foreground flex flex-wrap items-center gap-1.5 text-xs">
                      <Badge variant="secondary">{t(`company_branding.kind.${item.kind}`)}</Badge>
                      <span>{formatSize(item.size_bytes)}</span>
                    </div>
                    <p className="text-muted-foreground text-xs">
                      {item.usage.length
                        ? item.usage.map((u) => usageLabel(u, t)).join(" · ")
                        : t("company_branding.usage.none")}
                    </p>
                    <div className="flex gap-1">
                      <Button asChild variant="ghost" size="sm">
                        <a href={url} target="_blank" rel="noreferrer">
                          <ExternalLink className="size-4" />
                          {t("company_branding.library.open")}
                        </a>
                      </Button>
                      {canWrite ? (
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => void confirmRemove(item)}
                          disabled={remove.isPending || item.usage.length > 0}
                          title={item.usage.length ? t("company_branding.errors.in_use") : undefined}
                        >
                          <Trash2 className="size-4" />
                          {t("common.delete")}
                        </Button>
                      ) : null}
                    </div>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </EntitySectionCard>
  );
}
