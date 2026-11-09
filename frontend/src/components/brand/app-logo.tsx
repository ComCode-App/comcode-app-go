import { brand, type BrandTone, type BrandVariant } from "@/config/brand";
import { cn } from "@/lib/utils";

import { BrandLogoHorizontal, BrandMark } from "./brand-image";

type AppLogoProps = {
  variant?: BrandVariant;
  tone?: BrandTone;
  className?: string;
  width?: number;
  height?: number;
  priority?: boolean;
};

/**
 * ComCode lockup.
 * `mark` / `icon` → icon glyph; `logo` / `wordmark` → horizontal logo.
 */
export function AppLogo({
  variant = "logo",
  className,
  priority,
}: AppLogoProps) {
  const compact = variant === "mark" || variant === "icon";

  if (compact) {
    return (
      <BrandMark
        title={brand.name}
        className={cn("size-4 shrink-0", className)}
      />
    );
  }

  return (
    <BrandLogoHorizontal
      title={brand.productName}
      priority={priority}
      className={className}
    />
  );
}
