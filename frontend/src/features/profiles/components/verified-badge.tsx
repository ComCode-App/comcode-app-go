"use client";

import { BadgeCheck } from "lucide-react";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

/**
 * Platform-approved "verified profile" mark (TEC-549), shown next to a profile name.
 * It uses the `primary` token: every public theme keeps primary on card at WCAG AA
 * (themes.test.mjs), so the mark stays legible on all of them.
 */
export function VerifiedBadge({ className }: { className?: string }) {
  const { t } = useLocale();
  const label = t("public_profile.verified");
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          role="img"
          aria-label={label}
          tabIndex={0}
          className={cn(
            "text-primary focus-visible:ring-ring inline-flex shrink-0 items-center rounded-full align-middle outline-none focus-visible:ring-2",
            className,
          )}
        >
          <BadgeCheck aria-hidden className="size-[1em]" strokeWidth={2.25} />
        </span>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
