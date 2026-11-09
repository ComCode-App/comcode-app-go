"use client";

import { ChevronDown } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import {
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  useSidebar,
} from "@/components/ui/sidebar";
import { NavBadges } from "@/features/nav-engine/components/nav-badges";
import { useNavGroupOpen } from "@/features/nav-engine/hooks/use-nav-group-open";
import { isNavBranch } from "@/features/nav-engine/lib/access";
import { isNavHrefActive } from "@/features/nav-engine/lib/active";
import { mergeNavAdornment } from "@/features/nav-engine/lib/adornment";
import type { NavAdornment, NavItemDef } from "@/features/nav-engine/types";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

export function NavSidebarItem({
  item,
  adornment,
  homeHref,
  catalogId,
}: {
  item: NavItemDef;
  adornment: NavAdornment;
  homeHref: string;
  catalogId: string;
}) {
  const { t } = useLocale();
  const pathname = usePathname();
  const { isMobile, setOpenMobile } = useSidebar();

  if (isNavBranch(item)) {
    return (
      <NavSidebarBranchItem
        item={item}
        adornment={adornment}
        homeHref={homeHref}
        catalogId={catalogId}
      />
    );
  }

  const merged = mergeNavAdornment(item, adornment, t("common.soon"));
  const title = t(item.titleKey);
  const active = isNavHrefActive(pathname, item.href!, homeHref, item.exact);
  const Icon = item.icon;
  const tooltip = item.soon ? `${title} (${t("common.soon")})` : title;

  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        asChild
        isActive={active}
        tooltip={tooltip}
        className={cn(
          "transition-[background-color,box-shadow,transform] duration-200",
          active && "shadow-sm",
        )}
      >
        <Link
          href={item.href!}
          className={cn(item.soon && "opacity-90")}
          onClick={() => {
            if (isMobile) setOpenMobile(false);
          }}
        >
          <Icon />
          <span>{title}</span>
          <NavBadges
            badges={merged.badges}
            className="group-data-[collapsible=icon]:hidden"
          />
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function NavSidebarBranchItem({
  item,
  adornment,
  homeHref,
  catalogId,
}: {
  item: NavItemDef & { items: NavItemDef[] };
  adornment: NavAdornment;
  homeHref: string;
  catalogId: string;
}) {
  const { t } = useLocale();
  const pathname = usePathname();
  const merged = mergeNavAdornment(item, adornment, t("common.soon"));
  const title = t(item.titleKey);
  const Icon = item.icon;
  const hasActive = item.items.some((child) =>
    child.href ? isNavHrefActive(pathname, child.href, homeHref, child.exact) : false,
  );
  const [open, onOpenChange] = useNavGroupOpen(
    catalogId,
    `item:${item.id}`,
    hasActive || true,
  );

  return (
    <Collapsible
      open={open}
      onOpenChange={onOpenChange}
      className="group/nav-branch"
    >
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton isActive={hasActive} tooltip={title}>
            <Icon />
            <span>{title}</span>
            <ChevronDown className="ms-auto size-4 transition-transform duration-200 group-data-[state=closed]/nav-branch:-rotate-90 rtl:group-data-[state=closed]/nav-branch:rotate-90" />
            <NavBadges
              badges={merged.badges}
              className="group-data-[collapsible=icon]:hidden"
            />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent className="data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down overflow-hidden">
          <SidebarMenuSub>
            {item.items.map((child) => (
              <NavSidebarSubItem
                key={child.id}
                item={child}
                homeHref={homeHref}
              />
            ))}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
}

function NavSidebarSubItem({
  item,
  homeHref,
}: {
  item: NavItemDef;
  homeHref: string;
}) {
  const { t } = useLocale();
  const pathname = usePathname();
  const { isMobile, setOpenMobile } = useSidebar();
  const title = t(item.titleKey);
  const active = item.href
    ? isNavHrefActive(pathname, item.href, homeHref, item.exact)
    : false;
  const Icon = item.icon;

  return (
    <SidebarMenuSubItem>
      <SidebarMenuSubButton asChild isActive={active}>
        <Link
          href={item.href!}
          onClick={() => {
            if (isMobile) setOpenMobile(false);
          }}
        >
          <Icon />
          <span>{title}</span>
        </Link>
      </SidebarMenuSubButton>
    </SidebarMenuSubItem>
  );
}

export function NavDropdownItem({
  item,
  adornment,
  homeHref,
}: {
  item: NavItemDef;
  adornment: NavAdornment;
  homeHref: string;
}) {
  const { t } = useLocale();
  const pathname = usePathname();
  const { isMobile, setOpenMobile } = useSidebar();
  const merged = mergeNavAdornment(item, adornment, t("common.soon"));
  const title = t(item.titleKey);
  const active = item.href
    ? isNavHrefActive(pathname, item.href, homeHref, item.exact)
    : false;
  const Icon = item.icon;

  return (
    <DropdownMenuItem asChild>
      <Link
        href={item.href!}
        data-active={active}
        className={cn(active && "bg-accent")}
        onClick={() => {
          if (isMobile) setOpenMobile(false);
        }}
      >
        <Icon />
        <span className="min-w-0 flex-1 truncate">{title}</span>
        <NavBadges badges={merged.badges} compact />
      </Link>
    </DropdownMenuItem>
  );
}
