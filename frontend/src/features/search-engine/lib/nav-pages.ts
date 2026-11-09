import type { AppLayoutVariant } from "@/components/layout/app-layout";
import { appNav, platformNav } from "@/config/nav";
import {
  flattenNavItems,
  isNavEntryVisible,
  type NavAccessFns,
  visibleNavItems,
} from "@/features/nav-engine/lib/access";
import { resolveSearchIcon } from "@/features/search-engine/lib/icons";
import type { PaletteItem } from "@/features/search-engine/types";
import type { NavCatalog } from "@/features/nav-engine/types";
import { createElement } from "react";

export function buildNavPageItems(
  variant: AppLayoutVariant,
  access: NavAccessFns,
  t: (key: string) => string,
): PaletteItem[] {
  const catalog: NavCatalog = variant === "app" ? appNav : platformNav;
  const items: PaletteItem[] = [];

  for (const group of catalog.groups) {
    if (!isNavEntryVisible(group, access)) continue;
    const groupLabel = t(group.labelKey);
    for (const item of flattenNavItems(visibleNavItems(group, access))) {
      if (item.soon) continue;
      if (!item.href) continue;
      items.push({
        id: `page:${item.id}`,
        spec: "pages",
        label: t(item.titleKey),
        description: groupLabel,
        href: item.href,
        iconKey: item.searchIcon ?? "pages",
        icon: createElement(resolveSearchIcon(item.searchIcon ?? "pages"), {
          className: "size-4 shrink-0 opacity-80",
        }),
        group: t("search.group_pages"),
      });
    }
  }

  return items;
}
