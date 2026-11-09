"use client";

import { brand } from "@/config/brand";
import { BrandMark } from "@/components/brand/brand-image";
import { useLocale } from "@/providers/locale-provider";

const FOOTER_YEAR = new Date().getFullYear();

/** Theme-aware ComCode mark for muted footer surfaces. */
function FooterMark() {
  return (
    <BrandMark
      title={brand.productName}
      className="size-3.5 shrink-0 opacity-50 brightness-0 dark:opacity-75 dark:brightness-100"
    />
  );
}

export function Footer() {
  const { t } = useLocale();

  return (
    <footer className="border-border/60 bg-muted/40 mt-auto border-t px-4 py-2.5 backdrop-blur-sm md:px-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="text-muted-foreground flex items-center gap-2 text-xs">
          <FooterMark />
          <span className="text-foreground/75 font-medium tracking-tight">
            © {FOOTER_YEAR} {brand.productName}
          </span>
        </div>
        <p className="text-muted-foreground/80 pl-[1.375rem] text-[11px] leading-snug sm:pl-0 sm:text-right">
          {brand.tagline}
          <span className="text-muted-foreground/50 mx-1.5 hidden sm:inline">
            ·
          </span>
          <span className="hidden sm:inline">{t("layout.footer_rights")}</span>
        </p>
      </div>
    </footer>
  );
}
