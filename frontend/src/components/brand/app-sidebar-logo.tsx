import { brand } from "@/config/brand";
import { cn } from "@/lib/utils";

import { BrandMark } from "./brand-image";

type AppSidebarLogoProps = {
  className?: string;
};

/** Sidebar header lockup — icon tile + product name. */
export function AppSidebarLogo({ className }: AppSidebarLogoProps) {
  return (
    <>
      <span
        className={cn(
          "comcode-brand-gradient shadow-comcode-glow flex aspect-square size-8 items-center justify-center rounded-xl p-1.5",
          className,
        )}
      >
        <BrandMark className="size-full" title={brand.name} />
      </span>
      <span className="grid flex-1 text-left text-sm leading-tight">
        <span className="truncate font-medium">{brand.productName}</span>
        <span className="truncate text-xs opacity-70">{brand.tagline}</span>
      </span>
    </>
  );
}
