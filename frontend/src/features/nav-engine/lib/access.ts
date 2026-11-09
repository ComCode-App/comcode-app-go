import type { NavGroupDef, NavItemDef } from "@/features/nav-engine/types";

import type {
  NavAccountRole,
  NavAccountType,
} from "@/features/nav-engine/types";

export type NavAccessFns = {
  can: (permission: string | string[]) => boolean;
  canAny: (permissions: string[]) => boolean;
  /** Active account type; entries with `accountType` stay hidden when unknown. */
  accountType?: NavAccountType;
  /** Active account role; entries with `accountRoles` stay hidden when unknown. */
  accountRole?: NavAccountRole;
};

type AccessFns = NavAccessFns;

export function isNavEntryVisible(
  entry: Pick<
    NavItemDef,
    "permission" | "anyPermission" | "accountType" | "accountRoles"
  >,
  access: AccessFns,
): boolean {
  if (entry.accountType && entry.accountType !== access.accountType)
    return false;
  if (
    entry.accountRoles &&
    (!access.accountRole || !entry.accountRoles.includes(access.accountRole))
  ) {
    return false;
  }
  if (entry.permission && !access.can(entry.permission)) return false;
  if (entry.anyPermission?.length && !access.canAny(entry.anyPermission)) {
    return false;
  }
  return true;
}

export function isNavBranch(
  item: NavItemDef,
): item is NavItemDef & { items: NavItemDef[] } {
  return Boolean(item.items?.length);
}

function resolveVisibleNavItem(
  item: NavItemDef,
  access: AccessFns,
): NavItemDef | null {
  if (!isNavEntryVisible(item, access)) return null;
  if (isNavBranch(item)) {
    const children = item.items
      .map((child) => resolveVisibleNavItem(child, access))
      .filter((child): child is NavItemDef => child !== null);
    if (!children.length) return null;
    return { ...item, items: children };
  }
  if (!item.href) return null;
  return item;
}

export function visibleNavItems(group: NavGroupDef, access: AccessFns) {
  if (!isNavEntryVisible(group, access)) return [];
  return group.items
    .map((item) => resolveVisibleNavItem(item, access))
    .filter((item): item is NavItemDef => item !== null);
}

/** Flatten branch items for command palette / search indexing. */
export function flattenNavItems(items: NavItemDef[]): NavItemDef[] {
  const out: NavItemDef[] = [];
  for (const item of items) {
    if (isNavBranch(item)) {
      out.push(...flattenNavItems(item.items));
      continue;
    }
    if (item.href) out.push(item);
  }
  return out;
}
