"use client";

import { ArrowRight, BadgeCheck } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type PricingCardProps = {
  name: string;
  priceLabel: string;
  intervalLabel: string;
  description?: string | null;
  features: string[];
  ctaLabel: string;
  currentBadgeLabel?: string;
  popularBadgeLabel?: string;
  isCurrent?: boolean;
  isPopular?: boolean;
  isHighlighted?: boolean;
  isLoading?: boolean;
  disabled?: boolean;
  onSelect: () => void;
};

export function PricingCard({
  name,
  priceLabel,
  intervalLabel,
  description,
  features,
  ctaLabel,
  currentBadgeLabel,
  popularBadgeLabel,
  isCurrent = false,
  isPopular = false,
  isHighlighted = false,
  isLoading = false,
  disabled = false,
  onSelect,
}: PricingCardProps) {
  return (
    <div
      className={cn(
        "relative flex flex-col gap-8 overflow-hidden rounded-2xl border p-6 shadow-comcode-sm transition-shadow hover:shadow-comcode-md",
        isHighlighted
          ? "border-primary/30 bg-foreground text-background"
          : "bg-card text-card-foreground",
        isPopular &&
          !isHighlighted &&
          "outline outline-[color-mix(in_oklch,var(--comcode-magenta)_35%,transparent)]",
      )}
    >
      {isHighlighted ? <HighlightedBackground /> : null}
      {isPopular && !isHighlighted ? <PopularBackground /> : null}

      <div className="relative flex flex-wrap items-center gap-2">
        <h3 className="text-xl font-medium">{name}</h3>
        {isCurrent && currentBadgeLabel ? (
          <Badge variant="secondary" className="text-[10px]">
            {currentBadgeLabel}
          </Badge>
        ) : null}
        {isPopular && !isCurrent && popularBadgeLabel ? (
          <Badge className="comcode-brand-gradient border-0 text-[10px] text-primary-foreground hover:opacity-95">
            {popularBadgeLabel}
          </Badge>
        ) : null}
      </div>

      <div className="relative space-y-1">
        <p className="text-4xl font-semibold tracking-tight">{priceLabel}</p>
        <p
          className={cn(
            "text-xs font-medium",
            isHighlighted ? "text-background/70" : "text-muted-foreground",
          )}
        >
          {intervalLabel}
        </p>
      </div>

      <div className="relative flex-1 space-y-3">
        {description ? (
          <p
            className={cn(
              "text-sm font-medium",
              isHighlighted ? "text-background/90" : "text-foreground",
            )}
          >
            {description}
          </p>
        ) : null}
        {features.length > 0 ? (
          <ul className="space-y-2">
            {features.map((feature) => (
              <li
                key={feature}
                className={cn(
                  "flex items-start gap-2 text-sm font-medium",
                  isHighlighted ? "text-background/75" : "text-muted-foreground",
                )}
              >
                <BadgeCheck
                  className={cn(
                    "mt-0.5 shrink-0",
                    isHighlighted ? "text-primary-foreground" : "text-primary",
                  )}
                  strokeWidth={1.5}
                  size={16}
                />
                <span>{feature}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>

      <Button
        className={cn(
          "relative h-10 w-full rounded-lg",
          isHighlighted && "comcode-brand-gradient text-primary-foreground hover:brightness-110",
        )}
        variant={isHighlighted ? "default" : isCurrent ? "secondary" : "outline"}
        disabled={disabled || isLoading || isCurrent}
        onClick={onSelect}
      >
        {ctaLabel}
        {!isCurrent ? <ArrowRight className="size-4" /> : null}
      </Button>
    </div>
  );
}

function HighlightedBackground() {
  return (
    <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(to_right,color-mix(in_oklch,var(--comcode-magenta)_12%,transparent)_1px,transparent_1px),linear-gradient(to_bottom,color-mix(in_oklch,var(--comcode-magenta)_12%,transparent)_1px,transparent_1px)] bg-[size:45px_45px] opacity-100 [mask-image:radial-gradient(ellipse_80%_50%_at_50%_0%,#000_70%,transparent_110%)] dark:opacity-40" />
  );
}

function PopularBackground() {
  return (
    <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_80%_80%_at_50%_-20%,color-mix(in_oklch,var(--comcode-magenta)_18%,transparent),transparent)] dark:bg-[radial-gradient(ellipse_80%_80%_at_50%_-20%,color-mix(in_oklch,var(--comcode-magenta)_28%,transparent),transparent)]" />
  );
}
