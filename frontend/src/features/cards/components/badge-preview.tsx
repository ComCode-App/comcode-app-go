"use client";

import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";
import { DEFAULT_BADGE_ART, cardFaceImage } from "../lib/card-art";

/**
 * Static portrait (54×85.6 mm) preview of a badge (yaka kartı) for order screens (TEC-1320):
 * the design image, or the ComCode default artwork over the material colour, with the name to
 * be printed.
 */
export function BadgePreview({
  color = "#18181b",
  imageUrl,
  name,
  title,
  className,
}: {
  color?: string;
  imageUrl?: string | null;
  name?: string;
  title?: string;
  className?: string;
}) {
  const { t } = useLocale();
  const image = cardFaceImage(imageUrl);
  return (
    <div
      className={cn(
        "relative aspect-[54/85.6] overflow-hidden rounded-xl border border-white/10 shadow-2xl",
        className,
      )}
      style={{ backgroundColor: color }}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={image ?? DEFAULT_BADGE_ART.front}
        alt={image ? t("cards.art.front_alt") : ""}
        aria-hidden={image ? undefined : true}
        className="pointer-events-none absolute inset-0 h-full w-full object-cover"
      />
      {!image && (name || title) ? (
        <div className="absolute inset-x-2 top-[58%] text-center text-white">
          {name ? (
            <p className="truncate text-sm font-bold tracking-wide">{name}</p>
          ) : null}
          {title ? (
            <p className="truncate text-[11px] text-white/70">{title}</p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
