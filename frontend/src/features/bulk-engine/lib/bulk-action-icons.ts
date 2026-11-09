import {
  Contact,
  Image,
  Link2,
  Palette,
  Trash2,
  UserCheck,
  UserX,
  type LucideIcon,
} from "lucide-react";

import type { BulkActionDef, BulkActionMeta, BulkResource } from "@/features/bulk-engine/types";

const bulkActionIconCatalog: Record<
  BulkResource,
  Record<string, LucideIcon>
> = {
  "platform.users": {
    disable: UserX,
    enable: UserCheck,
  },
  "platform.roles": {
    delete: Trash2,
  },
  "platform.plans": {},
  "platform.subscriptions": {},
  "platform.companies": {},
  "app.company_profiles": {
    set_contact: Contact,
    set_logo: Image,
    set_theme: Palette,
    add_links: Link2,
  },
};

export function resolveBulkActionIcon(
  resource: BulkResource,
  actionId: string,
): LucideIcon {
  const icon = bulkActionIconCatalog[resource]?.[actionId];
  if (!icon) {
    throw new Error(
      `bulk-engine: missing icon for ${resource} action "${actionId}"`,
    );
  }
  return icon;
}

/** Attach mandatory icons to meta bulk actions for UI rendering. */
export function resolveBulkActionsWithIcons(
  resource: BulkResource,
  actions: BulkActionMeta[],
): BulkActionDef[] {
  return actions.map((action) => ({
    ...action,
    icon: resolveBulkActionIcon(resource, action.id),
  }));
}
