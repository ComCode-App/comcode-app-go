import type { LucideIcon } from "lucide-react";
import type { ComponentType, ReactNode } from "react";

export type NavBadgeVariant =
  "default" | "secondary" | "success" | "warning" | "danger" | "outline";

export type NavCountBadge = {
  kind: "count";
  value: number;
  variant?: NavBadgeVariant;
  max?: number;
  hiddenWhenZero?: boolean;
};

export type NavLabelBadge = {
  kind: "label";
  text: string;
  variant?: NavBadgeVariant;
};

export type NavDotBadge = {
  kind: "dot";
  variant?: NavBadgeVariant;
};

export type NavCustomBadge = {
  kind: "custom";
  id: string;
  render: () => ReactNode;
};

export type NavBadge =
  NavCountBadge | NavLabelBadge | NavDotBadge | NavCustomBadge;

export type NavInfoRow = {
  label: string;
  value: ReactNode;
  tone?: NavBadgeVariant;
};

export type NavInfo = {
  title?: string;
  description?: string;
  rows?: NavInfoRow[];
  /** Replaces the default title/rows panel when set. */
  render?: () => ReactNode;
};

export type NavAdornment = {
  badges?: NavBadge[];
  info?: NavInfo | null;
};

export type NavAdornmentComponent = ComponentType<{
  children: (adornment: NavAdornment) => ReactNode;
}>;

/** Active account type a nav entry is restricted to (docs/TENANCY_AND_ACCESS.md). */
export type NavAccountType = "personal" | "company";

/** Role of the user in the active account (docs/TENANCY_AND_ACCESS.md). */
export type NavAccountRole = "owner" | "admin" | "office_manager" | "member";

export type NavItemDef = {
  id: string;
  titleKey: string;
  /** Leaf items require href; branch items (with `items`) omit it. */
  href?: string;
  icon: LucideIcon;
  permission?: string | string[];
  anyPermission?: string[];
  /** Only visible while the active account has this type. */
  accountType?: NavAccountType;
  /** Only visible while the user holds one of these roles in the active account. */
  accountRoles?: NavAccountRole[];
  /** Active only on the exact href, not on nested paths owned by other items. */
  exact?: boolean;
  soon?: boolean;
  /** Command palette icon key (see resolveSearchIcon). */
  searchIcon?: string;
  /** Static badges merged after the live adornment. */
  badges?: NavBadge[];
  info?: NavInfo;
  Adornment?: NavAdornmentComponent;
  /** Nested sidebar links rendered under a collapsible parent. */
  items?: NavItemDef[];
};

export type NavGroupDef = {
  id: string;
  labelKey: string;
  /** Icon used as the collapsed (icon-mode) accordion trigger. */
  icon?: LucideIcon;
  defaultOpen?: boolean;
  collapsible?: boolean;
  permission?: string | string[];
  anyPermission?: string[];
  /** Only visible while the active account has this type. */
  accountType?: NavAccountType;
  /** Only visible while the user holds one of these roles in the active account. */
  accountRoles?: NavAccountRole[];
  items: NavItemDef[];
};

export type ResolvedNavItem = {
  item: NavItemDef;
  adornment: NavAdornment;
};

export type NavCatalog = {
  id: string;
  groups: NavGroupDef[];
};
