"use client";

import { Building2, Copy } from "lucide-react";
import { useEffect, useState, type CSSProperties } from "react";

import type { PublicProfile } from "@/features/public-profile/lib/load";
import {
  bankCardColors,
  dominantColor,
  parseHex,
  type RGB,
} from "@/features/public-profile/lib/bank-color";
import { formatIban } from "@/features/public-profile/lib/view";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

type BankAccount = PublicProfile["bank_accounts"][number];

const SAMPLE = 32;

/**
 * Dominant color of a bank logo, read in the browser. A separate CORS image is sampled so the
 * visible logo never depends on CORS; when the media host does not allow it (tainted canvas)
 * or the logo has no clear color, the result stays null.
 */
function useLogoColor(url: string | undefined): RGB | null {
  const [color, setColor] = useState<{ url: string; rgb: RGB | null } | null>(
    null,
  );
  useEffect(() => {
    if (!url) return;
    let cancelled = false;
    const img = new Image();
    img.crossOrigin = "anonymous";
    img.decoding = "async";
    img.onload = () => {
      if (cancelled) return;
      try {
        const canvas = document.createElement("canvas");
        canvas.width = SAMPLE;
        canvas.height = SAMPLE;
        const ctx = canvas.getContext("2d", { willReadFrequently: true });
        if (!ctx) return;
        ctx.drawImage(img, 0, 0, SAMPLE, SAMPLE);
        const rgb = dominantColor(ctx.getImageData(0, 0, SAMPLE, SAMPLE).data);
        setColor({ url, rgb });
      } catch {
        setColor({ url, rgb: null });
      }
    };
    img.onerror = () => {
      if (!cancelled) setColor({ url, rgb: null });
    };
    img.src = url;
    return () => {
      cancelled = true;
    };
  }, [url]);
  return color && color.url === url ? color.rgb : null;
}

/**
 * One published bank account on the public page (TEC-1314): a card in the bank's color (from
 * its logo, else its brand color) with copy buttons for the account holder and the IBAN.
 */
export function PublicBankAccount({
  account,
  onIbanCopy,
}: {
  account: BankAccount;
  onIbanCopy: () => void;
}) {
  const { t } = useLocale();
  const logoColor = useLogoColor(account.bank_icon_url || undefined);
  const base = logoColor ?? parseHex(account.bank_brand_color);
  const colors = base ? bankCardColors(base) : null;
  const style: CSSProperties | undefined = colors
    ? { background: colors.background, color: colors.foreground }
    : undefined;
  const chip: CSSProperties | undefined = colors
    ? { backgroundColor: colors.surface }
    : undefined;
  const muted: CSSProperties | undefined = colors
    ? { color: colors.muted }
    : undefined;

  const copy = (value: string, message: string) =>
    void navigator.clipboard
      .writeText(value)
      .then(() => appToast.success(message))
      .catch(() => undefined);

  return (
    <div
      className={
        colors
          ? "rounded-2xl p-4 shadow-sm"
          : "bg-muted/40 rounded-2xl border p-4"
      }
      style={style}
      data-bank-color={colors ? "brand" : "theme"}
    >
      <div className="flex items-center gap-3">
        <span className="flex size-11 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white shadow-xs">
          {account.bank_icon_url ? (
            // eslint-disable-next-line @next/next/no-img-element -- bank icons are API-owned
            <img
              src={account.bank_icon_url}
              alt=""
              className="size-8 object-contain"
              loading="lazy"
            />
          ) : (
            <Building2 className="size-5 text-slate-700" />
          )}
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold">
            {account.bank_name || account.title}
          </p>
          {account.bank_name && account.title ? (
            <p
              className={
                colors
                  ? "truncate text-xs"
                  : "text-muted-foreground truncate text-xs"
              }
              style={muted}
            >
              {account.title}
            </p>
          ) : null}
        </div>
      </div>

      {account.account_holder ? (
        <div className="mt-4 flex items-center gap-2">
          <div className="min-w-0 flex-1">
            <p
              className={
                colors ? "text-[11px]" : "text-muted-foreground text-[11px]"
              }
              style={muted}
            >
              {t("public_profile.account_holder")}
            </p>
            <p className="truncate text-sm font-medium">
              {account.account_holder}
            </p>
          </div>
          <button
            type="button"
            aria-label={t("public_profile.copy_account_holder")}
            onClick={() =>
              copy(
                account.account_holder ?? "",
                t("public_profile.account_holder_copied"),
              )
            }
            className={
              colors
                ? "flex size-9 shrink-0 items-center justify-center rounded-lg transition-opacity hover:opacity-80"
                : "bg-muted hover:bg-accent flex size-9 shrink-0 items-center justify-center rounded-lg"
            }
            style={chip}
          >
            <Copy className="size-4" />
          </button>
        </div>
      ) : null}

      <div className="mt-3 flex items-center gap-2">
        <code
          className={
            colors
              ? "min-w-0 flex-1 truncate rounded-lg px-3 py-2 text-xs tracking-wide"
              : "bg-muted min-w-0 flex-1 truncate rounded-lg px-3 py-2 text-xs tracking-wide"
          }
          style={chip}
        >
          {formatIban(account.iban)}
        </code>
        <button
          type="button"
          aria-label={t("public_profile.copy_iban")}
          onClick={() => {
            copy(
              account.iban.replace(/\s+/g, ""),
              t("public_profile.iban_copied"),
            );
            onIbanCopy();
          }}
          className={
            colors
              ? "flex size-9 shrink-0 items-center justify-center rounded-lg transition-opacity hover:opacity-80"
              : "bg-muted hover:bg-accent flex size-9 shrink-0 items-center justify-center rounded-lg"
          }
          style={chip}
        >
          <Copy className="size-4" />
        </button>
      </div>
    </div>
  );
}
