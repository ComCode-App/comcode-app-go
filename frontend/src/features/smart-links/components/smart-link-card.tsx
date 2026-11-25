"use client";

import {
  Contact,
  Copy,
  ExternalLink,
  Globe,
  Lock,
  LockOpen,
  Pencil,
  QrCode,
  Share2,
  Trash2,
  User,
  type LucideIcon,
} from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TooltipHint } from "@/components/ui/tooltip";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import type { SmartLink } from "@/features/smart-links/services/smart-links.service";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

const TARGET_ICONS: Record<string, LucideIcon> = {
  profile: User,
  social_hub: Share2,
  vcard_download: Contact,
  direct_url: Globe,
};

/** Background of the preview band, one per target type. */
const TARGET_GRADIENTS: Record<string, string> = {
  profile: "from-fuchsia-500 via-purple-500 to-indigo-500",
  social_hub: "from-sky-500 via-blue-500 to-violet-500",
  vcard_download: "from-emerald-500 via-teal-500 to-cyan-500",
  direct_url: "from-amber-500 via-orange-500 to-rose-500",
};

type SmartLinkCardProps = {
  link: SmartLink;
  url: string;
  profile?: ProfileSummary;
  canEdit: boolean;
  busy: boolean;
  onCopy: () => void;
  onQr: () => void;
  onEdit: () => void;
  onToggleLock: () => void;
  onDelete: () => void;
};

/** One smart link with a live preview of where it lands (TEC-1308). */
export function SmartLinkCard({
  link,
  url,
  profile,
  canEdit,
  busy,
  onCopy,
  onQr,
  onEdit,
  onToggleLock,
  onDelete,
}: SmartLinkCardProps) {
  const { t, locale } = useLocale();
  const locked = link.status === "locked";
  const TargetIcon = TARGET_ICONS[link.target_type] ?? Globe;
  const created = new Date(link.created_at).toLocaleDateString(
    locale === "tr" ? "tr-TR" : "en-US",
    { dateStyle: "medium" },
  );

  return (
    <li className="bg-card group flex flex-col overflow-hidden rounded-2xl border shadow-sm transition-shadow hover:shadow-md">
      <SmartLinkPreview link={link} profile={profile} locked={locked} />

      <div className="flex flex-1 flex-col gap-3 p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="block truncate text-base font-semibold hover:underline"
            >
              /{link.path}
            </a>
            <p className="text-muted-foreground mt-0.5 text-xs">
              {t("smart_links.card.created", { date: created })}
            </p>
          </div>
          <Badge
            variant="outline"
            className={cn(
              "shrink-0 gap-1.5",
              locked
                ? "text-muted-foreground"
                : "border-emerald-500/40 text-emerald-700 dark:text-emerald-400",
            )}
          >
            <span
              aria-hidden
              className={cn(
                "size-1.5 rounded-full",
                locked ? "bg-muted-foreground" : "bg-emerald-500",
              )}
            />
            {t(
              locked
                ? "smart_links.status.locked"
                : "smart_links.status.active",
            )}
          </Badge>
        </div>

        <div className="flex flex-wrap items-center gap-1.5 text-xs">
          <Badge variant="secondary" className="gap-1">
            <TargetIcon className="size-3" />
            {t(`smart_links.target.${link.target_type}`)}
          </Badge>
          {profile && link.target_type !== "direct_url" ? (
            <span className="text-muted-foreground truncate">
              {`${profile.first_name} ${profile.last_name}`.trim()}
            </span>
          ) : null}
        </div>

        <div className="bg-muted/50 flex items-center gap-2 rounded-lg border py-1 pr-1 pl-3">
          <span className="text-muted-foreground min-w-0 flex-1 truncate font-mono text-xs">
            {url.replace(/^https?:\/\//, "")}
          </span>
          <TooltipHint label={t("smart_links.copy")}>
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-7"
              aria-label={t("smart_links.copy")}
              onClick={onCopy}
            >
              <Copy className="size-3.5" />
            </Button>
          </TooltipHint>
        </div>

        <div className="mt-auto flex items-center justify-between gap-2 border-t pt-3">
          <Button asChild size="sm" variant="outline" className="gap-1.5">
            <a href={url} target="_blank" rel="noopener noreferrer">
              <ExternalLink className="size-3.5" />
              {t("smart_links.card.open")}
            </a>
          </Button>
          <div className="flex items-center gap-0.5">
            <IconAction label={t("smart_links.qr")} onClick={onQr}>
              <QrCode className="size-4" />
            </IconAction>
            {canEdit ? (
              <>
                <IconAction label={t("smart_links.edit")} onClick={onEdit}>
                  <Pencil className="size-4" />
                </IconAction>
                <IconAction
                  label={t(locked ? "smart_links.unlock" : "smart_links.lock")}
                  onClick={onToggleLock}
                  disabled={busy}
                >
                  {locked ? (
                    <LockOpen className="size-4" />
                  ) : (
                    <Lock className="size-4" />
                  )}
                </IconAction>
                <IconAction
                  label={t("smart_links.delete")}
                  onClick={onDelete}
                  disabled={busy}
                  className="text-destructive hover:text-destructive"
                >
                  <Trash2 className="size-4" />
                </IconAction>
              </>
            ) : null}
          </div>
        </div>
      </div>
    </li>
  );
}

function IconAction({
  label,
  onClick,
  disabled,
  className,
  children,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <TooltipHint label={label}>
      <Button
        type="button"
        size="icon"
        variant="ghost"
        className={cn("size-8", className)}
        aria-label={label}
        disabled={disabled}
        onClick={onClick}
      >
        {children}
      </Button>
    </TooltipHint>
  );
}

/** A small mock of the page the link opens, drawn from data the list already has. */
function SmartLinkPreview({
  link,
  profile,
  locked,
}: {
  link: SmartLink;
  profile?: ProfileSummary;
  locked: boolean;
}) {
  const { t } = useLocale();
  const name = profile
    ? `${profile.first_name} ${profile.last_name}`.trim()
    : t("smart_links.card.no_profile");
  const subline = profile
    ? [profile.title, profile.company_name].filter(Boolean).join(" · ")
    : "";
  const initials = name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");
  let host = "";
  let rest = "";
  if (link.target_type === "direct_url" && link.direct_url) {
    try {
      const parsed = new URL(link.direct_url);
      host = parsed.host;
      rest = `${parsed.pathname}${parsed.search}`.replace(/^\/$/, "");
    } catch {
      host = link.direct_url;
    }
  }

  return (
    <div
      className={cn(
        "relative flex h-40 items-end justify-center overflow-hidden bg-gradient-to-br px-6 pt-5",
        TARGET_GRADIENTS[link.target_type] ?? TARGET_GRADIENTS.direct_url,
      )}
    >
      <div
        aria-hidden
        className="absolute inset-0 bg-[radial-gradient(#ffffff26_1px,transparent_1px)] [background-size:14px_14px]"
      />
      <div
        className={cn(
          "bg-card text-card-foreground relative w-full max-w-60 rounded-t-2xl border border-b-0 px-4 pt-4 pb-3 shadow-lg transition-transform duration-300 group-hover:-translate-y-1",
          locked && "opacity-60 grayscale",
        )}
      >
        {link.target_type === "direct_url" ? (
          <div className="flex items-center gap-3">
            <span className="bg-muted flex size-10 shrink-0 items-center justify-center rounded-xl">
              <Globe className="text-muted-foreground size-5" />
            </span>
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold">{host || "—"}</p>
              <p className="text-muted-foreground truncate text-xs">
                {rest || "/"}
              </p>
            </div>
          </div>
        ) : (
          <div className="flex flex-col items-center text-center">
            <Avatar className="border-card -mt-9 size-12 border-2 shadow-sm">
              {profile?.avatar_url ? (
                <AvatarImage src={profile.avatar_url} alt="" />
              ) : null}
              <AvatarFallback className="text-xs font-semibold">
                {initials || <User className="size-4" />}
              </AvatarFallback>
            </Avatar>
            <p className="mt-1.5 max-w-full truncate text-sm font-semibold">
              {name}
            </p>
            {subline ? (
              <p className="text-muted-foreground max-w-full truncate text-[11px]">
                {subline}
              </p>
            ) : null}
            {link.target_type === "social_hub" ? (
              <div aria-hidden className="mt-2 flex gap-1.5">
                {[0, 1, 2, 3].map((i) => (
                  <span key={i} className="bg-muted size-5 rounded-full" />
                ))}
              </div>
            ) : link.target_type === "vcard_download" ? (
              <span className="bg-primary/10 text-primary mt-2 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium">
                <Contact className="size-3" />
                {t("smart_links.card.vcard_preview")}
              </span>
            ) : (
              <div aria-hidden className="mt-2 w-full space-y-1">
                <span className="bg-primary/80 block h-2.5 w-full rounded-full" />
                <span className="bg-muted block h-2.5 w-full rounded-full" />
              </div>
            )}
          </div>
        )}
      </div>
      {locked ? (
        <span className="bg-background/90 text-foreground absolute top-3 right-3 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium shadow-sm">
          <Lock className="size-3" />
          {t("smart_links.status.locked")}
        </span>
      ) : null}
    </div>
  );
}
