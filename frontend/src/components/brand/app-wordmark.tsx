import { brand } from "@/config/brand";
import { cn } from "@/lib/utils";

import { BrandLogoHorizontal } from "./brand-image";
import { BrandMark } from "./brand-image";

type AppWordmarkProps = {
  title?: string;
  className?: string;
};

/** ComCode horizontal logo — login / auth lockup. */
export function AppWordmark({
  className,
  title = brand.productName,
}: AppWordmarkProps) {
  return (
    <BrandLogoHorizontal
      title={title}
      priority
      className={cn("mx-auto", className)}
    />
  );
}

/** Compact mark tile + label — when horizontal logo is too wide. */
export function AppWordmarkCompact({
  className,
  title = brand.productName,
}: AppWordmarkProps) {
  return (
    <span
      className={cn("inline-flex items-center gap-2 font-medium", className)}
    >
      <span className="bg-primary flex size-8 items-center justify-center rounded-lg p-1.5">
        <BrandMark title={title} className="size-full" />
      </span>
      <span className="text-sm">{title}</span>
    </span>
  );
}
