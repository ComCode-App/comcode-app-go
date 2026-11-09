"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  PremiumLockIcon,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { assertServiceMediaURL } from "@/lib/media/urls";
import { cn } from "@/lib/utils";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { parseBackgroundDocument } from "../lib/json";
import {
  backgroundKeys,
  backgroundsService,
  type SavedBackground,
} from "../services/backgrounds.service";
import type { BackgroundDocument } from "../types";

type Props = {
  activeUuid: string | null;
  canEdit: boolean;
  onLoad: (doc: BackgroundDocument, saved: SavedBackground) => void;
  onDeleted: (uuid: string) => void;
};

/** Account designs and gallery presets. Loading one puts it into the editor. */
export function SavedBackgroundsPanel({
  activeUuid,
  canEdit,
  onLoad,
  onDeleted,
}: Props) {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const queryClient = useQueryClient();
  const { data = [] } = useQuery({
    queryKey: backgroundKeys.list,
    queryFn: backgroundsService.list,
  });
  const allowPremium = useDesignPremium();

  const remove = useMutation({
    mutationFn: backgroundsService.remove,
    onSuccess: (_res, uuid) => {
      void queryClient.invalidateQueries({ queryKey: backgroundKeys.list });
      appToast.success(t("background_studio.toast.deleted"));
      onDeleted(uuid);
    },
  });

  const load = (item: SavedBackground) => {
    try {
      onLoad(parseBackgroundDocument(item.document), item);
    } catch {
      appToast.error(t("background_studio.toast.invalid_json"));
    }
  };

  return (
    <div className="bg-card rounded-xl border p-4">
      <h2 className="mb-3 text-sm font-semibold">
        {t("background_studio.saved.title")}
      </h2>
      {data.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          {t("background_studio.saved.empty")}
        </p>
      ) : (
        <ul className="divide-y">
          {data.map((item) => (
            <li key={item.uuid} className="flex items-center gap-2 py-2">
              <button
                type="button"
                onClick={() => load(item)}
                className={cn(
                  "hover:bg-muted/60 flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1 text-left text-sm",
                  item.uuid === activeUuid && "bg-muted",
                )}
              >
                <BackgroundThumb url={item.thumbnail_url} />
                <span className="truncate font-medium">{item.name}</span>
                {item.premium && !allowPremium ? <PremiumLockIcon /> : null}
                <Badge variant="outline" className="shrink-0 capitalize">
                  {item.kind}
                </Badge>
                {item.is_preset ? (
                  <Badge variant="secondary" className="shrink-0">
                    {t("background_studio.saved.preset")}
                  </Badge>
                ) : null}
                {item.uuid === activeUuid ? (
                  <span className="text-primary shrink-0 text-xs">
                    {t("background_studio.saved.editing")}
                  </span>
                ) : null}
              </button>
              {canEdit && !item.is_preset ? (
                <Button
                  type="button"
                  size="icon"
                  variant="ghost"
                  aria-label={t("background_studio.actions.delete")}
                  disabled={remove.isPending}
                  onClick={() =>
                    void confirmDelete({
                      title: t("background_studio.delete_title"),
                      description: t("background_studio.delete_description", {
                        name: item.name,
                      }),
                    }).then((ok) => ok && remove.mutate(item.uuid))
                  }
                >
                  <Trash2 className="size-4" />
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** Static thumbnail of a saved design (TEC-599); a neutral square until it is generated. */
function BackgroundThumb({ url }: { url: string | null }) {
  const src = assertServiceMediaURL(url);
  return src ? (
    // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, tiny decorative preview
    <img
      src={src}
      alt=""
      loading="lazy"
      className="size-8 shrink-0 rounded-md border object-cover"
    />
  ) : (
    <span aria-hidden className="bg-muted size-8 shrink-0 rounded-md border" />
  );
}
