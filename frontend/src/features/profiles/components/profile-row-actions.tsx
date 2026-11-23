"use client";

import { Download, Eye, Pencil, Star, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";
import { TooltipHint } from "@/components/ui/tooltip";

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
        <TooltipHint label={t("profiles.actions.view")}>
          <Button
            variant="ghost"
            size="sm"
            className="text-muted-foreground hover:text-foreground h-8 w-8 p-0"
            onClick={() => handlers.onView!(profile)}
            aria-label={t("profiles.actions.view")}
          >
            <Eye className="h-4 w-4" />
            <span className="sr-only">{t("profiles.actions.view")}</span>
          </Button>
        </TooltipHint>
      )}

      <TooltipHint label={t("profiles.actions.edit")}>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground hover:text-foreground h-8 w-8 p-0"
          onClick={() => handlers.onEdit(profile)}
          aria-label={t("profiles.actions.edit")}
        >
          <Pencil className="h-4 w-4" />
          <span className="sr-only">{t("profiles.actions.edit")}</span>
        </Button>
      </TooltipHint>

      <TooltipHint label={t("profiles.actions.download_vcard")}>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground hover:text-foreground h-8 w-8 p-0"
          onClick={() => handlers.onDownloadVCard(profile)}
          aria-label={t("profiles.actions.download_vcard")}
        >
          <Download className="h-4 w-4" />
          <span className="sr-only">
            {t("profiles.actions.download_vcard")}
          </span>
        </Button>
      </TooltipHint>

      {!profile.is_primary && (
        <TooltipHint label={t("profiles.actions.set_primary")}>
          <Button
            variant="ghost"
            size="sm"
            className="text-muted-foreground h-8 w-8 p-0 hover:text-amber-500"
            onClick={() => handlers.onSetPrimary(profile)}
            aria-label={t("profiles.actions.set_primary")}
          >
            <Star className="h-4 w-4" />
            <span className="sr-only">{t("profiles.actions.set_primary")}</span>
          </Button>
        </TooltipHint>
      )}

      <TooltipHint label={t("profiles.actions.delete")}>
        <Button
          variant="ghost"
          size="sm"
          className="text-muted-foreground hover:text-destructive h-8 w-8 p-0"
          onClick={() => handlers.onDelete(profile)}
          aria-label={t("profiles.actions.delete")}
        >
          <Trash2 className="h-4 w-4" />
          <span className="sr-only">{t("profiles.actions.delete")}</span>
        </Button>
      </TooltipHint>
    </div>
  );
}
