"use client";

import { Download, Eye, Pencil, Star, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

export type ProfileRowActionHandlers = {
  onView?: (profile: ProfileSummary) => void;
  onEdit: (profile: ProfileSummary) => void;
  onDownloadVCard: (profile: ProfileSummary) => void;
  onSetPrimary: (profile: ProfileSummary) => void;
  onDelete: (profile: ProfileSummary) => void;
};

export function ProfileRowActionsMenu({
  profile,
  handlers,
}: {
  profile: ProfileSummary;
  handlers: ProfileRowActionHandlers;
}) {
  const { t } = useLocale();

  return (
    <div
      className="flex items-center justify-end gap-1"
      onClick={(e) => e.stopPropagation()}
    >
      {handlers.onView && (
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground hover:text-foreground h-8 w-8 p-0"
          onClick={() => handlers.onView!(profile)}
          title={t("profiles.actions.view")}
        >
          <Eye className="h-4 w-4" />
          <span className="sr-only">{t("profiles.actions.view")}</span>
        </Button>
      )}

      <Button
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground h-8 w-8 p-0"
        onClick={() => handlers.onEdit(profile)}
        title={t("profiles.actions.edit")}
      >
        <Pencil className="h-4 w-4" />
        <span className="sr-only">{t("profiles.actions.edit")}</span>
      </Button>

      <Button
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-foreground h-8 w-8 p-0"
        onClick={() => handlers.onDownloadVCard(profile)}
        title={t("profiles.actions.download_vcard")}
      >
        <Download className="h-4 w-4" />
        <span className="sr-only">{t("profiles.actions.download_vcard")}</span>
      </Button>

      {!profile.is_primary && (
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground h-8 w-8 p-0 hover:text-amber-500"
          onClick={() => handlers.onSetPrimary(profile)}
          title={t("profiles.actions.set_primary")}
        >
          <Star className="h-4 w-4" />
          <span className="sr-only">{t("profiles.actions.set_primary")}</span>
        </Button>
      )}

      <Button
        variant="ghost"
        size="sm"
        className="text-muted-foreground hover:text-destructive h-8 w-8 p-0"
        onClick={() => handlers.onDelete(profile)}
        title={t("profiles.actions.delete")}
      >
        <Trash2 className="h-4 w-4" />
        <span className="sr-only">{t("profiles.actions.delete")}</span>
      </Button>
    </div>
  );
}
