"use client";

import { useRef, useState } from "react";
import { X } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { FileInput } from "@/components/ui/file-input";
import { cardFaceImage, type CardSide } from "@/features/cards/lib/card-art";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { ordersService } from "../services/orders.service";

const MAX_BYTES = 5 * 1024 * 1024;

const SIDE_LABEL: Record<CardSide, string> = {
  front: "orders.card_images.front",
  back: "orders.card_images.back",
};

/**
 * Picks one card face's design image for an order the platform places (TEC-1320). The image is
 * uploaded right away; the order payload carries the returned URL.
 */
export function CardImagePicker({
  side,
  value,
  onChange,
}: {
  side: CardSide;
  value?: string;
  onChange: (url: string | undefined) => void;
}) {
  const { t } = useLocale();
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const src = cardFaceImage(value);

  const pick = async (file: File) => {
    if (file.size > MAX_BYTES) {
      toast.error(t("orders.card_images.too_large"));
      return;
    }
    setBusy(true);
    try {
      const res = await ordersService.uploadCardImage(file);
      onChange(res.url);
    } catch (err) {
      // API errors are already shown by the global API error toast.
      if (!isApiError(err)) toast.error(t("orders.card_images.upload_failed"));
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  if (src) {
    return (
      <div className="flex items-center gap-1">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={src}
          alt={t(SIDE_LABEL[side])}
          className="h-7 w-11 rounded border object-cover"
        />
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="h-6 w-6"
          aria-label={t("orders.card_images.remove")}
          onClick={() => onChange(undefined)}
        >
          <X className="h-3 w-3" />
        </Button>
      </div>
    );
  }
  return (
    <FileInput
      ref={inputRef}
      accept="image/png,image/jpeg,image/webp"
      showFileName={false}
      disabled={busy}
      buttonSize="sm"
      className="[&_button]:h-7 [&_button]:px-2 [&_button]:text-[11px]"
      buttonLabel={t(SIDE_LABEL[side])}
      onChange={(e) => {
        const file = e.target.files?.[0];
        if (file) void pick(file);
      }}
    />
  );
}
