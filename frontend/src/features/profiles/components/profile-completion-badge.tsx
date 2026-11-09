"use client";

import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

interface ProfileCompletionBadgeProps {
  rate: number;
  showBar?: boolean;
  className?: string;
}

export function ProfileCompletionBadge({
  rate,
  showBar = true,
  className,
}: ProfileCompletionBadgeProps) {
  const { t } = useLocale();
  const clamped = Math.max(0, Math.min(100, rate));

  const getColorClass = (val: number) => {
    if (val >= 80) return "text-emerald-600 dark:text-emerald-400";
    if (val >= 50) return "text-amber-600 dark:text-amber-400";
    return "text-rose-600 dark:text-rose-400";
  };

  return (
    <div className={cn("flex flex-col gap-1 min-w-[90px]", className)}>
      <div className="flex items-center justify-between text-xs">
        <span className="text-muted-foreground font-medium">
          {t("profiles.completion.rate_label")}
        </span>
        <span className={cn("font-semibold", getColorClass(clamped))}>
          %{clamped}
        </span>
      </div>
      {showBar && (
        <Progress
          value={clamped}
          className="h-1.5 w-full bg-muted/60"
        />
      )}
    </div>
  );
}
