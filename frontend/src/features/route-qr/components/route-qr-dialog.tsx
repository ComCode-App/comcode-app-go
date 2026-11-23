"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Check,
  Copy,
  Download,
  Info,
  Loader2,
  QrCode,
  RotateCcw,
  Server,
} from "lucide-react";
import QRCode from "qrcode";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { permissions } from "@/config/permissions";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { triggerBrowserDownload } from "@/lib/api/platform-form-request";
import { canManageAccount } from "@/lib/auth/types";
import { cn } from "@/lib/utils";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

import {
  darkColors,
  MIN_CONTRAST,
  normalizeQrStyle,
  qrContrast,
  qrLogoBox,
  qrStyleProblems,
  type QrStyle,
  type QrStyleProblem,
  renderQrSvg,
} from "../lib/qr-engine";
import {
  fetchAsDataUrl,
  svgBlob,
  svgDataUrl,
  svgToPngBlob,
} from "../lib/qr-export";
import {
  DEFAULT_QR_STYLE,
  DOWNLOAD_FORMATS,
  DOWNLOAD_SIZES,
  type DownloadFormat,
  matrixFor,
  readabilityWarnings,
} from "../lib/qr-style";
import {
  type RouteQR,
  type RouteQRStyle,
  routeQrKeys,
  routeQrService,
} from "../services/route-qr.service";
import { QrDesignControls } from "./qr-design-controls";

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

/** Smart QR designer (TEC-569, TEC-1270): templates, shapes, colors, logo, downloads. */
export function RouteQrDialog({
  routeUuid,
  title,
  open,
  onOpenChange,
  suggestedLogoProfileUuid,
  fileStem,
}: RouteQrDialogProps) {
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
      <DialogContent className="max-h-[94vh] overflow-y-auto p-4 sm:max-w-5xl sm:p-6">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <QrCode className="text-primary size-5" />
            <span className="truncate">{title}</span>
          </DialogTitle>
          <DialogDescription>{t("routes.qr.subtitle")}</DialogDescription>
        </DialogHeader>

        {!routeUuid || !canRead ? (
          <p className="text-muted-foreground text-sm">
            {t("routes.qr.unavailable")}
          </p>
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
          <p className="text-destructive text-sm">
            {t("routes.qr.load_failed")}
          </p>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** Fallback only: an older API without matrices. The mask may differ from the server's. */
function localMatrix(payload: string, hasLogo: boolean): string[] | null {
  try {
    const m = QRCode.create(payload, {
      errorCorrectionLevel: hasLogo ? "H" : "M",
    }).modules;
    const rows: string[] = [];
    for (let y = 0; y < m.size; y++) {
      let row = "";
      for (let x = 0; x < m.size; x++) row += m.get(y, x) ? "1" : "0";
      rows.push(row);
    }
    return rows;
  } catch {
    return null;
  }
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

  const saved = useMemo(() => normalizeQrStyle(qr.style), [qr.style]);
  const [draft, setDraft] = useState<QrStyle>(saved);
  const full = useMemo(() => normalizeQrStyle(draft), [draft]);
  const dirty = JSON.stringify(full) !== JSON.stringify(saved);
  const problems = useMemo(() => qrStyleProblems(draft), [draft]);
  const set = (patch: Partial<QrStyle>) =>
    setDraft((d) => ({ ...d, ...patch }));

  // Logo: only media the API owns — an account profile's avatar or cover, picked by reference.
  const profilesQuery = useQuery({
    queryKey: profilesKeys.list({ picker: true }),
    queryFn: () => profilesService.list({ limit: 100, offset: 0 }),
    enabled: canEdit || Boolean(full.logo_profile_uuid),
  });
  const logoProfileUuid = full.logo_profile_uuid;
  const logoProfileQuery = useQuery({
    queryKey: profilesKeys.detail(logoProfileUuid ?? ""),
    queryFn: () => profilesService.get(logoProfileUuid!),
    enabled: Boolean(logoProfileUuid),
  });
  const logoMedia = {
    avatar: logoProfileQuery.data?.avatar_url || undefined,
    cover: logoProfileQuery.data?.cover_url || undefined,
  };
  const logoUrl =
    full.logo_field === "avatar" || full.logo_field === "cover"
      ? logoMedia[full.logo_field]
      : undefined;
  const logoMissing =
    Boolean(logoProfileUuid) && logoProfileQuery.isSuccess && !logoUrl;
  // The logo as a data: URL makes browser files self-contained; null if the media is unreadable.
  const logoData = useQuery({
    queryKey: ["route-qr", "logo-data", logoUrl ?? ""],
    queryFn: ({ signal }) => fetchAsDataUrl(logoUrl!, signal),
    enabled: Boolean(logoUrl),
    staleTime: 5 * 60_000,
    retry: false,
  });

  // The server's own modules (M, or H with a logo): the local SVG equals /qr/render.
  const hasLogo = Boolean(logoProfileUuid);
  const serverMatrix = matrixFor(qr.matrices, hasLogo);
  const matrix = useMemo(
    () => serverMatrix ?? localMatrix(qr.payload, hasLogo),
    [serverMatrix, qr.payload, hasLogo],
  );
  const drawable = !problems.some(
    (p) =>
      p.kind === "color_format" || p.kind === "shape" || p.kind === "gradient",
  );
  const previewSvg = useMemo(
    () =>
      matrix && drawable
        ? renderQrSvg(matrix, full, 512, logoData.data ?? "")
        : null,
    [matrix, drawable, full, logoData.data],
  );
  const logoBox =
    matrix && hasLogo && logoUrl && !logoData.data
      ? qrLogoBox(matrix.length, full)
      : null;

  const bg = full.transparent_background ? "#ffffff" : full.background;
  const contrasts = darkColors(full)
    .map((c) => qrContrast(c.color, bg))
    .filter((c): c is number => c !== null);
  const warnings = readabilityWarnings(full, contrasts);

  const save = useMutation({
    // Saving is only enabled without problems, so the enums hold.
    mutationFn: () => routeQrService.put(routeUuid, full as RouteQRStyle),
    onSuccess: (next) => {
      queryClient.setQueryData(routeQrKeys.detail(routeUuid), next);
      appToast.success(t("routes.qr.saved"));
    },
  });

  const [format, setFormat] = useState<DownloadFormat>("png");
  const [size, setSize] = useState<number>(1024);
  const stem = fileStem || "qr";
  // svg/png of the current draft are drawn here; PDF and saved-design files come from the API.
  const localBlocked =
    !matrix ||
    !drawable ||
    problems.length > 0 ||
    (hasLogo && (!logoData.data || logoMissing));
  const localDownload = useMutation({
    mutationFn: async () => {
      const svg = renderQrSvg(matrix!, full, size, logoData.data ?? "");
      return format === "svg" ? svgBlob(svg) : svgToPngBlob(svg, size);
    },
    onSuccess: (blob) => triggerBrowserDownload(blob, `${stem}-qr.${format}`),
    onError: () => appToast.error(t("routes.qr.download_failed")),
  });
  const serverDownload = useMutation({
    mutationFn: () => routeQrService.render(routeUuid, format, size),
    onSuccess: ({ blob, filename }) =>
      triggerBrowserDownload(blob, filename ?? `${stem}-qr.${format}`),
  });

  const [copied, setCopied] = useState(false);
  const copyPayload = () =>
    void navigator.clipboard.writeText(qr.payload).then(() => {
      setCopied(true);
      appToast.success(t("routes.qr.payload_copied"));
      window.setTimeout(() => setCopied(false), 2000);
    });

  return (
    <div className="space-y-6">
      <div className="grid gap-6 md:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        {/* Preview + encoded URL */}
        <div className="min-w-0 space-y-4">
          <div className="bg-muted/40 rounded-xl border p-3 sm:p-4 md:sticky md:top-0">
            <div
              className={cn(
                "relative mx-auto aspect-square w-full max-w-56 overflow-hidden rounded-lg shadow-sm sm:max-w-80",
                full.transparent_background &&
                  "bg-[repeating-conic-gradient(#e5e7eb_0_25%,#fff_0_50%)] bg-[length:16px_16px]",
              )}
            >
              {previewSvg ? (
                <>
                  {/* eslint-disable-next-line @next/next/no-img-element -- data URL of the engine's SVG */}
                  <img
                    src={svgDataUrl(previewSvg)}
                    alt={t("routes.qr.preview_alt")}
                    className="size-full"
                  />
                  {logoBox ? (
                    // Media the browser cannot inline is shown on top; files then come from the API.
                    // eslint-disable-next-line @next/next/no-img-element -- API-owned profile media URL
                    <img
                      src={logoUrl}
                      alt=""
                      className="absolute object-contain"
                      style={{
                        left: `${(logoBox.x / logoBox.total) * 100}%`,
                        top: `${(logoBox.y / logoBox.total) * 100}%`,
                        width: `${(logoBox.w / logoBox.total) * 100}%`,
                        height: `${(logoBox.w / logoBox.total) * 100}%`,
                      }}
                    />
                  ) : null}
                </>
              ) : (
                <div className="bg-muted flex size-full items-center justify-center p-6 text-center text-sm">
                  {t("routes.qr.preview_invalid")}
                </div>
              )}
            </div>
            <p className="text-muted-foreground mt-3 text-center text-xs">
              {serverMatrix
                ? dirty
                  ? t("routes.qr.preview_unsaved")
                  : t("routes.qr.preview_saved")
                : t("routes.qr.preview_sketch")}
              {" · "}
              {t("routes.qr.error_correction", { level: hasLogo ? "H" : "M" })}
            </p>
            <QrChecks problems={problems} warnings={warnings} />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor={`${ids}-payload`}>{t("routes.qr.payload")}</Label>
            <div className="flex gap-2">
              <Input
                id={`${ids}-payload`}
                readOnly
                value={qr.payload}
                className="bg-muted/50 font-mono text-xs"
              />
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={copyPayload}
                title={t("routes.qr.copy_payload")}
              >
                {copied ? (
                  <Check className="size-4" />
                ) : (
                  <Copy className="size-4" />
                )}
                <span className="sr-only">{t("routes.qr.copy_payload")}</span>
              </Button>
            </div>
            <p className="text-muted-foreground flex gap-1.5 text-xs">
              <Info className="mt-0.5 size-3.5 shrink-0" />
              {t("routes.qr.payload_hint")}
            </p>
          </div>
        </div>

        <QrDesignControls
          style={full}
          set={set}
          canEdit={canEdit}
          disabled={!canEdit || save.isPending}
          matrix={matrix}
          profiles={profilesQuery.data?.items ?? []}
          suggestedLogoProfileUuid={suggestedLogoProfileUuid}
          logoMedia={logoMedia}
          logoLoading={logoProfileQuery.isLoading}
          logoMissing={logoMissing}
        />
      </div>

      <DialogFooter className="flex-col gap-3 border-t pt-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-wrap items-end gap-2">
          <div className="grid gap-1.5">
            <Label className="text-xs">{t("routes.qr.format")}</Label>
            <Select
              value={format}
              onValueChange={(v) => setFormat(v as DownloadFormat)}
            >
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
            <Label className="text-xs">
              {format === "svg" ? t("routes.qr.size_svg") : t("routes.qr.size")}
            </Label>
            <Select
              value={String(size)}
              onValueChange={(v) => setSize(Number(v))}
            >
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
          {format !== "pdf" ? (
            <Button
              type="button"
              variant="outline"
              onClick={() => localDownload.mutate()}
              disabled={localBlocked || localDownload.isPending}
              title={t("routes.qr.download_local_hint")}
            >
              {localDownload.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                <Download className="size-4" />
              )}
              {t("routes.qr.download")}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="outline"
            onClick={() => serverDownload.mutate()}
            disabled={dirty || serverDownload.isPending}
            title={
              dirty
                ? t("routes.qr.download_save_first")
                : t("routes.qr.download_server_hint")
            }
          >
            {serverDownload.isPending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Server className="size-4" />
            )}
            {t("routes.qr.download_server")}
          </Button>
          <p className="text-muted-foreground w-full text-xs">
            {hasLogo && !logoData.data && !logoData.isLoading
              ? t("routes.qr.download_logo_server")
              : dirty
                ? t("routes.qr.download_dirty_hint")
                : t("routes.qr.download_local_hint")}
          </p>
        </div>

        {canEdit ? (
          <div className="flex gap-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                setDraft(dirty ? saved : normalizeQrStyle(DEFAULT_QR_STYLE))
              }
              disabled={
                save.isPending ||
                (!dirty &&
                  JSON.stringify(saved) ===
                    JSON.stringify(normalizeQrStyle(DEFAULT_QR_STYLE)))
              }
            >
              <RotateCcw className="size-4" />
              {dirty ? t("routes.qr.discard") : t("routes.qr.reset_defaults")}
            </Button>
            <Button
              type="button"
              onClick={() => save.mutate()}
              disabled={
                !dirty || problems.length > 0 || logoMissing || save.isPending
              }
            >
              {save.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : null}
              {t("routes.qr.save")}
            </Button>
          </div>
        ) : null}
      </DialogFooter>
    </div>
  );
}

function problemText(
  t: (key: string, vars?: Record<string, string | number>) => string,
  p: QrStyleProblem,
  formatRatio: (r: number) => string,
): string {
  const where = (role: string, eye?: number) =>
    eye === undefined
      ? t(`routes.qr.role_${role}`)
      : t("routes.qr.role_eye", {
          part: t(`routes.qr.role_${role}`),
          eye: t(`routes.qr.eye_name_${eye}`),
        });
  switch (p.kind) {
    case "color_format":
      return t("routes.qr.problem_color_format", {
        where: where(p.role, p.eye),
      });
    case "inverted":
      return t("routes.qr.problem_inverted", { where: where(p.role, p.eye) });
    case "contrast":
      return t("routes.qr.problem_contrast", {
        where: where(p.role, p.eye),
        ratio: formatRatio(p.ratio),
        min: MIN_CONTRAST,
      });
    default:
      return t(`routes.qr.problem_${p.kind}`);
  }
}

/** Hard problems (the API rejects the style) and readability advice. */
function QrChecks({
  problems,
  warnings,
}: {
  problems: QrStyleProblem[];
  warnings: string[];
}) {
  const { t, locale } = useLocale();
  // Rounded down so a failing 4.46 never reads as "4.5:1".
  const formatRatio = (r: number) =>
    `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(Math.floor(r * 10) / 10)}:1`;
  if (problems.length === 0 && warnings.length === 0)
    return (
      <p
        role="status"
        className="mt-3 flex items-center justify-center gap-1.5 text-xs text-emerald-700 dark:text-emerald-400"
      >
        <Check className="size-3.5" />
        {t("routes.qr.checks_ok")}
      </p>
    );
  return (
    <ul role="status" className="mt-3 space-y-1.5 text-xs">
      {problems.map((p, i) => (
        <li
          key={`p${i}`}
          className="border-destructive/40 bg-destructive/5 flex gap-1.5 rounded-md border p-2"
        >
          <AlertTriangle className="text-destructive mt-0.5 size-3.5 shrink-0" />
          {problemText(t, p, formatRatio)}
        </li>
      ))}
      {warnings.map((w) => (
        <li
          key={w}
          className="flex gap-1.5 rounded-md border border-amber-500/40 bg-amber-500/5 p-2"
        >
          <Info className="mt-0.5 size-3.5 shrink-0 text-amber-600" />
          {t(`routes.qr.warning_${w}`)}
        </li>
      ))}
    </ul>
  );
}
