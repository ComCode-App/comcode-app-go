"use client";

import { Check, ImageOff } from "lucide-react";
import Link from "next/link";

import { Label } from "@/components/ui/label";
import type { SavedBackground } from "@/features/background-studio/services/backgrounds.service";
import {
  PremiumLockIcon,
  PremiumNotice,
} from "@/features/entitlements/components/premium-lock";
import { routes } from "@/config/routes";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/** Static image of a design: its rendered thumbnail, else a video's poster. */
export function backgroundImage(b: SavedBackground | undefined): string | null {
  if (!b) return null;
  if (b.thumbnail_url) return b.thumbnail_url;
  if (b.document.kind === "video") return b.document.params.poster ?? null;
  return null;
}

type Props = {
  backgrounds: SavedBackground[];
  /** Selected design; null = the template's own backdrop. */
  value: string | null;
  /** Design stored on the server: stays pickable even when the plan locks it (TEC-947). */
  storedValue?: string | null;
  allowPremium: boolean;
  disabled?: boolean;
  onChange: (uuid: string | null) => void;
};

/**
 * Background Studio gallery for the public page (TEC-1309): the account's designs and the
 * platform presets as preview tiles. The background replaces only the template's backdrop.
 */
export function BackgroundTilePicker({
  backgrounds,
  value,
  storedValue,
  allowPremium,
  disabled,
  onChange,
}: Props) {
  const { t } = useLocale();
  const selected = backgrounds.find((b) => b.uuid === value);
  const hasLocked = !allowPremium && backgrounds.some((b) => b.premium);
  const tiles = [
    { uuid: null, name: t("profiles.public_page.background_default") },
    ...backgrounds,
  ];

  return (
    <div className="grid gap-1.5">
      <Label id="public-page-background-label">
        {t("profiles.public_page.background")}
      </Label>
      <div
        role="radiogroup"
        aria-labelledby="public-page-background-label"
        className="grid max-h-80 grid-cols-3 gap-2 overflow-y-auto p-0.5 sm:grid-cols-5"
      >
        {tiles.map((tile) => {
          const bg = "kind" in tile ? tile : undefined;
          const isSelected = tile.uuid === value;
          const locked =
            Boolean(bg?.premium) && !allowPremium && tile.uuid !== storedValue;
          const image = backgroundImage(bg);
          return (
            <button
              key={tile.uuid ?? "default"}
              type="button"
              role="radio"
              aria-checked={isSelected}
              aria-disabled={locked ? true : undefined}
              disabled={disabled}
              onClick={() => {
                if (!locked) onChange(tile.uuid);
              }}
              className={cn(
                "flex flex-col gap-1 rounded-lg border p-1 text-left transition-colors",
                "focus-visible:ring-ring/50 outline-none focus-visible:ring-[3px] disabled:opacity-60",
                isSelected
                  ? "border-primary ring-primary/30 ring-2"
                  : locked
                    ? "cursor-not-allowed opacity-60"
                    : "hover:border-foreground/30",
              )}
            >
              <span className="bg-muted relative flex aspect-[9/14] items-center justify-center overflow-hidden rounded-md">
                {image ? (
                  // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL
                  <img
                    src={image}
                    alt=""
                    className="absolute inset-0 size-full object-cover"
                    loading="lazy"
                  />
                ) : bg ? (
                  <ImageOff className="text-muted-foreground size-4" />
                ) : (
                  <span className="text-muted-foreground px-1 text-center text-[10px] leading-tight">
                    {t("profiles.public_page.background_template")}
                  </span>
                )}
              </span>
              <span className="flex items-center justify-between gap-1 px-0.5 text-[11px] font-medium">
                <span className="truncate">{tile.name}</span>
                {isSelected ? (
                  <Check className="text-primary size-3 shrink-0" />
                ) : locked ? (
                  <PremiumLockIcon />
                ) : null}
              </span>
            </button>
          );
        })}
      </div>
      <p className="text-muted-foreground text-xs">
        {t("profiles.public_page.background_hint")}{" "}
        <Link
          href={routes.app.backgroundStudio.root}
          className="text-primary hover:underline"
        >
          {t("profiles.public_page.background_studio_link")}
        </Link>
      </p>
      {selected?.premium && !allowPremium ? (
        <p className="text-xs text-amber-700 dark:text-amber-400">
          {t("profiles.public_page.background_premium_fallback")}
        </p>
      ) : null}
      {hasLocked ? <PremiumNotice /> : null}
    </div>
  );
}
