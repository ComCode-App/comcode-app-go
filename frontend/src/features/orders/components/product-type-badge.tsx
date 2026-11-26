"use client";

import { Badge } from "@/components/ui/badge";
import { productShapeOf } from "@/features/cards/lib/card-art";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/** Marks a badge (yaka kartı) product in order screens (TEC-1320); nothing for a card. */
export function ProductTypeBadge({
  productType,
  className,
}: {
  productType?: string | null;
  className?: string;
}) {
  const { t } = useLocale();
  if (productShapeOf(productType) !== "badge") return null;
  return (
    <Badge
      variant="outline"
      className={cn("h-4 px-1.5 py-0 text-[10px]", className)}
    >
      {t("cards.types.badge")}
    </Badge>
  );
}
