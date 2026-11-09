"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { AlertTriangle, Check, Copy, Download, Info, Loader2, QrCode, RotateCcw } from "lucide-react";
import { useId, useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { permissions } from "@/config/permissions";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { useDebounce } from "@/hooks/use-debounce";
import { ApiError } from "@/lib/api/errors";
import { triggerBrowserDownload } from "@/lib/api/platform-form-request";
import { canManageAccount } from "@/lib/auth/types";
import { cn } from "@/lib/utils";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

import {
  DEFAULT_QR_STYLE,
  DOWNLOAD_FORMATS,
  DOWNLOAD_SIZES,
  type DownloadFormat,
  EYE_STYLES,
  isHexColor,
  MIN_CONTRAST,
  MODULE_STYLES,
  normalizeStyle,
  previewGeometry,
  QUIET_ZONE_MAX,
  QUIET_ZONE_MIN,
  qrContrast,
  sameStyle,
  styleProblem,
} from "../lib/qr-style";
import { type RouteQR, type RouteQRStyle, routeQrKeys, routeQrService } from "../services/route-qr.service";

type RouteQrDialogProps = {
  /** Route whose identity the QR encodes (card c/ route, smart link, canonical /{username}). */
  routeUuid: string | null | undefined;
  /** Shown as the dialog title, e.g. "/s/promo" or the card name. */
  title: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Profile suggested first when the owner turns the logo on (e.g. the profile being edited). */
  suggestedLogoProfileUuid?: string;
  /** File name stem for downloads when the API sends none. */
  fileStem?: string;
};

/** Smart QR designer (TEC-569): appearance, live preview, save and server-rendered downloads. */
export function RouteQrDialog({ routeUuid, title, open, onOpenChange, suggestedLogoProfileUuid, fileStem }: RouteQrDialogProps) {
  const { t } = useLocale();
  const { can } = usePermission();
  const canRead = can(permissions.routes.appRead);

  const qrQuery = useQuery({
    queryKey: routeQrKeys.detail(routeUuid ?? ""),
    queryFn: () => routeQrService.get(routeUuid!),
    enabled: open && Boolean(routeUuid) && canRead,
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92vh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <QrCode className="text-primary size-5" />
            <span className="truncate">{title}</span>
          </DialogTitle>
          <DialogDescription>{t("routes.qr.subtitle")}</DialogDescription>
        </DialogHeader>

        {!routeUuid || !canRead ? (
          <p className="text-muted-foreground text-sm">{t("routes.qr.unavailable")}</p>
        ) : qrQuery.isLoading ? (
          <div className="flex h-64 items-center justify-center">
            <Loader2 className="text-muted-foreground size-6 animate-spin" />
          </div>
        ) : qrQuery.data ? (
          // Remount on every saved state so the draft starts from the server's values.
          <QrDesigner
            key={JSON.stringify(qrQuery.data.style)}
            routeUuid={routeUuid}
            qr={qrQuery.data}
            suggestedLogoProfileUuid={suggestedLogoProfileUuid}
            fileStem={fileStem}
          />
        ) : (
          <p className="text-destructive text-sm">{t("routes.qr.load_failed")}</p>
        )}
      </DialogContent>
    </Dialog>
  );
}

function QrDesigner({
  routeUuid,
  qr,
  suggestedLogoProfileUuid,
  fileStem,
}: {
  routeUuid: string;
  qr: RouteQR;
  suggestedLogoProfileUuid?: string;
  fileStem?: string;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const { can } = usePermission();
  // Writes need app.routes.write and an owner/admin account role (the API checks both).
  const canEdit = can(permissions.routes.appWrite) && canManageAccount(user);
  const queryClient = useQueryClient();
  const ids = useId();

  const saved = useMemo(() => normalizeStyle(qr.style), [qr.style]);
  const [draft, setDraft] = useState<RouteQRStyle>(saved);
  const dirty = !sameStyle(draft, saved);
  const problem = styleProblem(draft);
  const set = (patch: Partial<RouteQRStyle>) => setDraft((d) => ({ ...d, ...patch }));

  // Logo: only media the API owns — an account profile's avatar or cover, picked by reference.
  const profilesQuery = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
    enabled: canEdit || Boolean(draft.logo_profile_uuid),
  });
  const logoProfileUuid = draft.logo_profile_uuid ?? null;
  const logoProfileQuery = useQuery({
    queryKey: profilesKeys.detail(logoProfileUuid ?? ""),
    queryFn: () => profilesService.get(logoProfileUuid!),
    enabled: Boolean(logoProfileUuid),
  });
  const logoMedia = {
    avatar: logoProfileQuery.data?.avatar_url || undefined,
    cover: logoProfileQuery.data?.cover_url || undefined,
  };
  const logoUrl = draft.logo_field === "avatar" || draft.logo_field === "cover" ? logoMedia[draft.logo_field] : undefined;
  const profileOptions = profilesQuery.data?.items ?? [];
  const logoMissing = Boolean(logoProfileUuid) && logoProfileQuery.isSuccess && !logoUrl;

  const pickLogoProfile = (value: string) => {
    if (value === "none") {
      set({ logo_profile_uuid: null, logo_field: "" });
      return;
    }
    set({ logo_profile_uuid: value, logo_field: draft.logo_field || "avatar" });
  };

  // Preview (TEC-954): the API renders the current draft with the same renderer as downloads and
  // the print pack, debounced because it is rate limited. The local drawing is only an instant
  // sketch while that render is pending (or if it fails); its bit pattern may differ.
  const sketch = useDebounce(draft, 120);
  const geometry = useMemo(() => previewGeometry(qr.payload, sketch), [qr.payload, sketch]);
  const settledDraft = useDebounce(draft, 400);
  const settledStyle = useMemo(() => normalizeStyle(settledDraft), [settledDraft]);
  const signature = JSON.stringify(settledStyle);
  const settled = signature === JSON.stringify(normalizeStyle(draft));
  const serverPreview = useQuery({
    queryKey: routeQrKeys.preview(routeUuid, signature),
    queryFn: async ({ signal }) => {
      try {
        const svg = await (await routeQrService.preview(routeUuid, settledStyle, "svg", 512, signal)).text();
        return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`;
      } catch (err) {
        // Shown inline below the preview; keep it out of the global error toast.
        throw new Error(err instanceof ApiError ? err.message : String(err));
      }
    },
    // Styles the API would reject anyway are not sent; the contrast/logo hints explain why.
    enabled: styleProblem(settledStyle) === null && !logoMissing,
    staleTime: 5 * 60_000,
    retry: false,
  });
  const serverUrl = settled ? serverPreview.data : undefined;
  const serverFailed = settled && serverPreview.isError;

  const save = useMutation({
    mutationFn: () => routeQrService.put(routeUuid, normalizeStyle(draft)),
    onSuccess: (next) => {
      queryClient.setQueryData(routeQrKeys.detail(routeUuid), next);
      appToast.success(t("routes.qr.saved"));
    },
  });

  const [format, setFormat] = useState<DownloadFormat>("png");
  const [size, setSize] = useState<number>(1024);
  const download = useMutation({
    mutationFn: () => routeQrService.render(routeUuid, format, size),
    onSuccess: ({ blob, filename }) => {
      triggerBrowserDownload(blob, filename ?? `${fileStem || "qr"}-qr.${format}`);
    },
  });

  const [copied, setCopied] = useState(false);
  const copyPayload = () =>
    void navigator.clipboard.writeText(qr.payload).then(() => {
      setCopied(true);
      appToast.success(t("routes.qr.payload_copied"));
      window.setTimeout(() => setCopied(false), 2000);
    });

  const ratio = qrContrast(draft.foreground, draft.background);
  const errorCorrection = draft.logo_profile_uuid ? "H" : "M";

  return (
    <div className="space-y-6">
      <div className="grid gap-6 md:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
        {/* Preview + encoded URL */}
        <div className="space-y-4">
          <div className="bg-muted/40 rounded-xl border p-4">
            <div className="mx-auto aspect-square w-full max-w-80 overflow-hidden rounded-lg shadow-sm">
              {serverUrl ? (
                // eslint-disable-next-line @next/next/no-img-element -- data URL of the API-rendered SVG
                <img src={serverUrl} alt={t("routes.qr.preview_alt")} className="size-full" />
              ) : geometry ? (
                <svg
                  viewBox={`0 0 ${geometry.total} ${geometry.total}`}
                  className="size-full"
                  role="img"
                  aria-label={t("routes.qr.preview_alt")}
                >
                  <rect width="100%" height="100%" fill={sketch.background} />
                  {geometry.shapes.map((s, i) => (
                    <rect key={i} x={s.x} y={s.y} width={s.w} height={s.w} rx={s.r} fill={s.fill} />
                  ))}
                  {geometry.logo && logoUrl ? (
                    <image
                      x={geometry.logo.pos}
                      y={geometry.logo.pos}
                      width={geometry.logo.w}
                      height={geometry.logo.w}
                      href={logoUrl}
                      preserveAspectRatio="xMidYMid meet"
                    />
                  ) : null}
                </svg>
              ) : (
                <div className="bg-muted flex size-full items-center justify-center p-6 text-center text-sm">
                  {t("routes.qr.preview_invalid")}
                </div>
              )}
            </div>
            <p className="text-muted-foreground mt-3 text-center text-xs">
              {!serverUrl && geometry && !serverFailed
                ? t("routes.qr.preview_sketch")
                : dirty
                  ? t("routes.qr.preview_unsaved")
                  : t("routes.qr.preview_saved")}
              {" · "}
              {t("routes.qr.error_correction", { level: errorCorrection })}
            </p>
            {serverFailed ? (
              <p className="text-destructive mt-1 text-center text-xs">{t("routes.qr.preview_server_failed")}</p>
            ) : null}
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor={`${ids}-payload`}>{t("routes.qr.payload")}</Label>
            <div className="flex gap-2">
              <Input id={`${ids}-payload`} readOnly value={qr.payload} className="bg-muted/50 font-mono text-xs" />
              <Button type="button" variant="outline" size="icon" onClick={copyPayload} title={t("routes.qr.copy_payload")}>
                {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                <span className="sr-only">{t("routes.qr.copy_payload")}</span>
              </Button>
            </div>
            <p className="text-muted-foreground flex gap-1.5 text-xs">
              <Info className="mt-0.5 size-3.5 shrink-0" />
              {t("routes.qr.payload_hint")}
            </p>
          </div>
        </div>

        {/* Controls */}
        <fieldset disabled={!canEdit || save.isPending} className="min-w-0 space-y-5">
          {!canEdit ? <p className="text-muted-foreground text-sm">{t("routes.qr.read_only")}</p> : null}

          <div className="grid gap-4 sm:grid-cols-2">
            <ColorField
              id={`${ids}-fg`}
              label={t("routes.qr.foreground")}
              value={draft.foreground}
              onChange={(foreground) => set({ foreground })}
            />
            <ColorField
              id={`${ids}-bg`}
              label={t("routes.qr.background")}
              value={draft.background}
              onChange={(background) => set({ background })}
            />
          </div>

          <ContrastIndicator ratio={ratio} problem={problem} />

          <div className="grid gap-1.5">
            <Label>{t("routes.qr.module")}</Label>
            <ToggleGroup
              type="single"
              variant="outline"
              value={draft.module}
              onValueChange={(v) => v && set({ module: v as RouteQRStyle["module"] })}
              disabled={!canEdit}
            >
              {MODULE_STYLES.map((m) => (
                <ToggleGroupItem key={m} value={m}>
                  {t(`routes.qr.module_${m}`)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>

          <div className="grid gap-1.5">
            <Label>{t("routes.qr.eye")}</Label>
            <ToggleGroup
              type="single"
              variant="outline"
              value={draft.eye}
              onValueChange={(v) => v && set({ eye: v as RouteQRStyle["eye"] })}
              disabled={!canEdit}
            >
              {EYE_STYLES.map((e) => (
                <ToggleGroupItem key={e} value={e}>
                  {t(`routes.qr.eye_${e}`)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>

          <div className="grid gap-2">
            <div className="flex items-center justify-between">
              <Label htmlFor={`${ids}-quiet`}>{t("routes.qr.quiet_zone")}</Label>
              <span className="text-muted-foreground text-xs tabular-nums">
                {t("routes.qr.quiet_zone_value", { count: draft.quiet_zone })}
              </span>
            </div>
            <Slider
              id={`${ids}-quiet`}
              min={QUIET_ZONE_MIN}
              max={QUIET_ZONE_MAX}
              step={1}
              value={[draft.quiet_zone]}
              onValueChange={([v]) => set({ quiet_zone: v ?? QUIET_ZONE_MIN })}
              disabled={!canEdit}
            />
            <p className="text-muted-foreground text-xs">{t("routes.qr.quiet_zone_hint")}</p>
          </div>

          <div className="grid gap-2">
            <Label>{t("routes.qr.logo")}</Label>
            <Select
              value={logoProfileUuid ?? "none"}
              onValueChange={pickLogoProfile}
              disabled={!canEdit}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="none">{t("routes.qr.logo_none")}</SelectItem>
                {profileOptions
                  .slice()
                  .sort((a, b) => Number(b.uuid === suggestedLogoProfileUuid) - Number(a.uuid === suggestedLogoProfileUuid))
                  .map((p) => (
                    <SelectItem key={p.uuid} value={p.uuid}>
                      {`${p.first_name} ${p.last_name}`.trim() || p.username || p.uuid}
                    </SelectItem>
                  ))}
                {logoProfileUuid && !profileOptions.some((p) => p.uuid === logoProfileUuid) ? (
                  <SelectItem value={logoProfileUuid}>{t("routes.qr.logo_profile_unknown")}</SelectItem>
                ) : null}
              </SelectContent>
            </Select>
            {logoProfileUuid ? (
              <div className="grid grid-cols-2 gap-2">
                {(["avatar", "cover"] as const).map((field) => {
                  const url = logoMedia[field];
                  const active = draft.logo_field === field;
                  return (
                    <button
                      key={field}
                      type="button"
                      aria-pressed={active}
                      disabled={!canEdit || (!url && !logoProfileQuery.isLoading)}
                      onClick={() => set({ logo_field: field })}
                      className={cn(
                        "flex items-center gap-2 rounded-lg border p-2 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                        active ? "border-primary ring-primary/30 ring-2" : "hover:bg-muted",
                      )}
                    >
                      <span className="bg-muted size-10 shrink-0 overflow-hidden rounded-md">
                        {url ? (
                          // eslint-disable-next-line @next/next/no-img-element -- API-owned profile media URL
                          <img src={url} alt="" className="size-full object-cover" />
                        ) : null}
                      </span>
                      <span className="min-w-0">
                        <span className="block font-medium">{t(`routes.qr.logo_${field}`)}</span>
                        {!url && !logoProfileQuery.isLoading ? (
                          <span className="text-muted-foreground block text-xs">{t("routes.qr.logo_missing")}</span>
                        ) : null}
                      </span>
                    </button>
                  );
                })}
              </div>
            ) : null}
            {logoMissing ? <p className="text-destructive text-xs">{t("routes.qr.logo_pick_media")}</p> : null}
            <p className="text-muted-foreground text-xs">{t("routes.qr.logo_hint")}</p>
          </div>
        </fieldset>
      </div>

      <DialogFooter className="flex-col gap-3 border-t pt-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-wrap items-end gap-2">
          <div className="grid gap-1.5">
            <Label className="text-xs">{t("routes.qr.format")}</Label>
            <Select value={format} onValueChange={(v) => setFormat(v as DownloadFormat)}>
              <SelectTrigger className="w-28">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DOWNLOAD_FORMATS.map((f) => (
                  <SelectItem key={f} value={f}>
                    {t(`routes.qr.format_${f}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="grid gap-1.5">
            <Label className="text-xs">{format === "svg" ? t("routes.qr.size_svg") : t("routes.qr.size")}</Label>
            <Select value={String(size)} onValueChange={(v) => setSize(Number(v))}>
              <SelectTrigger className="w-32">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DOWNLOAD_SIZES.map((s) => (
                  <SelectItem key={s} value={String(s)}>
                    {t("routes.qr.size_px", { size: s })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <Button
            type="button"
            variant="outline"
            onClick={() => download.mutate()}
            disabled={dirty || download.isPending}
            title={dirty ? t("routes.qr.download_save_first") : undefined}
          >
            {download.isPending ? <Loader2 className="size-4 animate-spin" /> : <Download className="size-4" />}
            {t("routes.qr.download")}
          </Button>
          {dirty ? <p className="text-muted-foreground w-full text-xs">{t("routes.qr.download_save_first")}</p> : null}
        </div>

        {canEdit ? (
          <div className="flex gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => setDraft(dirty ? saved : DEFAULT_QR_STYLE)}
              disabled={save.isPending || (!dirty && sameStyle(saved, DEFAULT_QR_STYLE))}
            >
              <RotateCcw className="size-4" />
              {dirty ? t("routes.qr.discard") : t("routes.qr.reset_defaults")}
            </Button>
            <Button type="button" onClick={() => save.mutate()} disabled={!dirty || problem !== null || logoMissing || save.isPending}>
              {save.isPending ? <Loader2 className="size-4 animate-spin" /> : null}
              {t("routes.qr.save")}
            </Button>
          </div>
        ) : null}
      </DialogFooter>
    </div>
  );
}

function ColorField({
  id,
  label,
  value,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const valid = isHexColor(value);
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex items-center gap-2">
        <input
          type="color"
          aria-label={label}
          value={valid ? value.toLowerCase() : "#000000"}
          onChange={(e) => onChange(e.target.value)}
          className="border-input size-9 shrink-0 cursor-pointer rounded-md border bg-transparent p-0.5 disabled:cursor-not-allowed"
        />
        <Input
          id={id}
          value={value}
          onChange={(e) => {
            const v = e.target.value.trim();
            onChange(v.startsWith("#") ? v : `#${v}`);
          }}
          maxLength={7}
          spellCheck={false}
          autoComplete="off"
          aria-invalid={!valid}
          className="font-mono uppercase"
        />
      </div>
    </div>
  );
}

function ContrastIndicator({ ratio, problem }: { ratio: number | null; problem: ReturnType<typeof styleProblem> }) {
  const { t, locale } = useLocale();
  const colorProblem = problem === "color_format" || problem === "contrast" || problem === "inverted";
  // Rounded down so a failing 4.46 never reads as "4.5:1".
  const display = ratio === null ? "–" : `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(Math.floor(ratio * 10) / 10)}:1`;
  return (
    <div
      role="status"
      className={cn(
        "flex items-start gap-2 rounded-lg border p-3 text-sm",
        colorProblem ? "border-destructive/40 bg-destructive/5" : "bg-muted/40",
      )}
    >
      {colorProblem ? (
        <AlertTriangle className="text-destructive mt-0.5 size-4 shrink-0" />
      ) : (
        <Check className="mt-0.5 size-4 shrink-0 text-emerald-600" />
      )}
      <div className="min-w-0">
        <p className="font-medium">
          {t("routes.qr.contrast", { ratio: display })}
          {" · "}
          {colorProblem ? t("routes.qr.contrast_fail") : t("routes.qr.contrast_pass")}
        </p>
        <p className="text-muted-foreground text-xs">
          {problem === "color_format"
            ? t("routes.qr.contrast_invalid_color")
            : problem === "inverted"
              ? t("routes.qr.contrast_inverted")
              : t("routes.qr.contrast_hint", { min: MIN_CONTRAST })}
        </p>
      </div>
    </div>
  );
}
