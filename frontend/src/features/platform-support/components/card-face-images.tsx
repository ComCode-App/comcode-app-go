"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ImageOff } from "lucide-react";
import { useRef } from "react";

import { Button } from "@/components/ui/button";
import { FileInput } from "@/components/ui/file-input";
import {
  DEFAULT_CARD_ART,
  cardFaceImage,
  type CardSide,
} from "@/features/cards/lib/card-art";
import {
  type SupportCard,
  platformSupportKeys,
  platformSupportService,
} from "@/features/platform-support/services/platform-support.service";
import { useStepUp } from "@/features/step-up-engine";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const MAX_BYTES = 5 * 1024 * 1024;

const SIDE_LABEL: Record<CardSide, string> = {
  front: "routes.support.card_images.front",
  back: "routes.support.card_images.back",
};
const SIDE_ALT: Record<CardSide, string> = {
  front: "cards.art.front_alt",
  back: "cards.art.back_alt",
};
const ACCEPT = "image/png,image/jpeg,image/webp";

/**
 * Front and back design images of a card (TEC-1320). Without an image the card shows the
 * ComCode default artwork. Images of another ratio are cropped to the card (object-fit: cover).
 */
export function CardFaceImages({
  card,
  canWrite,
}: {
  card: SupportCard;
  canWrite: boolean;
}) {
  const { t } = useLocale();
  return (
    <div className="space-y-2">
      <p className="text-muted-foreground text-xs">
        {t("routes.support.card_images.hint")}
      </p>
      <div className="grid gap-3 sm:grid-cols-2">
        {(["front", "back"] as const).map((side) => (
          <Face key={side} card={card} side={side} canWrite={canWrite} />
        ))}
      </div>
    </div>
  );
}

function Face({
  card,
  side,
  canWrite,
}: {
  card: SupportCard;
  side: CardSide;
  canWrite: boolean;
}) {
  const { t } = useLocale();
  const { ensure } = useStepUp();
  const queryClient = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);
  const image = cardFaceImage(
    side === "front" ? card.front_image_url : card.back_image_url,
  );

  const onError = (err: unknown) =>
    appToast.error(
      isApiError(err) || err instanceof Error
        ? err.message
        : t("common.error_generic"),
    );
  const onDone = (key: string) => {
    void queryClient.invalidateQueries({ queryKey: platformSupportKeys.all });
    appToast.success(t(key));
  };

  const upload = useMutation({
    mutationFn: async (file: File) => {
      if (!(await ensure()))
        throw new Error(t("routes.support.step_up_cancelled"));
      return platformSupportService.uploadCardImage(card.uuid, side, file);
    },
    onSuccess: () => onDone("routes.support.card_images.uploaded"),
    onError,
    onSettled: () => {
      if (inputRef.current) inputRef.current.value = "";
    },
  });
  const remove = useMutation({
    mutationFn: async () => {
      if (!(await ensure()))
        throw new Error(t("routes.support.step_up_cancelled"));
      return platformSupportService.removeCardImage(card.uuid, side);
    },
    onSuccess: () => onDone("routes.support.card_images.removed"),
    onError,
  });
  const busy = upload.isPending || remove.isPending;

  return (
    <div className="space-y-2">
      <p className="text-xs font-medium">{t(SIDE_LABEL[side])}</p>
      <div className="relative aspect-[1.586/1] w-full overflow-hidden rounded-lg border bg-gradient-to-br from-zinc-900 via-neutral-900 to-black">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={image ?? DEFAULT_CARD_ART[side]}
          alt={
            image ? t(SIDE_ALT[side]) : t("routes.support.card_images.default")
          }
          className="h-full w-full object-cover"
        />
        {!image ? (
          <span className="absolute top-2 left-2 rounded bg-black/50 px-1.5 py-0.5 text-[10px] text-white/80">
            {t("routes.support.card_images.default")}
          </span>
        ) : null}
      </div>
      {canWrite ? (
        <div className="flex flex-wrap items-center gap-2">
          <FileInput
            ref={inputRef}
            accept={ACCEPT}
            showFileName={false}
            disabled={busy}
            buttonLabel={t(
              image
                ? "routes.support.card_images.replace"
                : "routes.support.card_images.upload",
            )}
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (!file) return;
              if (file.size > MAX_BYTES) {
                appToast.error(t("routes.support.card_images.too_large"));
                e.target.value = "";
                return;
              }
              upload.mutate(file);
            }}
          />
          {image ? (
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() => remove.mutate()}
            >
              <ImageOff className="mr-1.5 size-4" />
              {t("routes.support.card_images.remove")}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
