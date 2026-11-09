"use client";

import React, { useState, useEffect } from "react";
import QRCode from "qrcode";
import {
  Lock,
  Wifi,
  Sparkles,
  User,
  ExternalLink,
  Share2,
  RotateCw,
  QrCode as QrCodeIcon,
  Settings,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { CardSummary } from "../services/cards.service";
import { useLocale } from "@/providers/locale-provider";

interface InteractiveNFCCardProps {
  card: CardSummary;
  onSettingsClick?: () => void;
  onQRClick?: () => void;
  onToggleLock?: () => void;
  className?: string;
}

const MATERIAL_STYLES: Record<
  string,
  {
    bg: string;
    text: string;
    subtext: string;
    accent: string;
    border: string;
    chip: string;
    label: string;
  }
> = {
  matte_black: {
    bg: "bg-gradient-to-br from-zinc-900 via-neutral-900 to-black",
    text: "text-white",
    subtext: "text-zinc-400",
    accent: "text-amber-400",
    border: "border-zinc-800 shadow-2xl shadow-black/80",
    chip: "from-amber-200/80 to-amber-600/90 border-amber-400/60",
    label: "Mat Siyah",
  },
  metal_gold: {
    bg: "bg-gradient-to-br from-amber-600 via-yellow-700 to-amber-950",
    text: "text-amber-100",
    subtext: "text-amber-200/80",
    accent: "text-yellow-200",
    border: "border-yellow-500/50 shadow-2xl shadow-amber-950/50",
    chip: "from-yellow-100 to-yellow-500 border-yellow-300",
    label: "Altın Metal",
  },
  metal_silver: {
    bg: "bg-gradient-to-br from-slate-700 via-zinc-800 to-slate-950",
    text: "text-slate-100",
    subtext: "text-slate-300",
    accent: "text-cyan-300",
    border: "border-slate-500/50 shadow-2xl shadow-slate-950/60",
    chip: "from-slate-200 to-slate-400 border-slate-300",
    label: "Gümüş Çelik",
  },
  rose_gold: {
    bg: "bg-gradient-to-br from-rose-800 via-pink-900 to-zinc-950",
    text: "text-rose-100",
    subtext: "text-rose-200/80",
    accent: "text-rose-300",
    border: "border-rose-400/40 shadow-2xl shadow-rose-950/50",
    chip: "from-rose-200 to-rose-400 border-rose-300",
    label: "Rose Gold",
  },
  midnight_blue: {
    bg: "bg-gradient-to-br from-blue-900 via-indigo-950 to-slate-950",
    text: "text-blue-100",
    subtext: "text-blue-300",
    accent: "text-sky-300",
    border: "border-blue-600/40 shadow-2xl shadow-blue-950/60",
    chip: "from-blue-200 to-indigo-400 border-blue-300",
    label: "Gece Mavisi",
  },
  emerald: {
    bg: "bg-gradient-to-br from-emerald-900 via-teal-950 to-neutral-950",
    text: "text-emerald-100",
    subtext: "text-emerald-300",
    accent: "text-emerald-300",
    border: "border-emerald-600/40 shadow-2xl shadow-emerald-950/60",
    chip: "from-emerald-200 to-teal-400 border-emerald-300",
    label: "Zümrüt Yeşili",
  },
};

export function InteractiveNFCCard({
  card,
  onSettingsClick,
  onQRClick,
  onToggleLock,
  className,
}: InteractiveNFCCardProps) {
  const { t } = useLocale();
  const [isFlipped, setIsFlipped] = useState(false);
  const [qrDataUrl, setQrDataUrl] = useState<string>("");

  const style = MATERIAL_STYLES[card.material] || MATERIAL_STYLES.matte_black;
  const matKey = `cards.materials.${card.material}`;
  const matTranslated = t(matKey);
  const materialLabel = matTranslated.startsWith("cards.") ? style.label : matTranslated;

  const targetUrl = typeof window !== "undefined"
    ? `${window.location.origin}/c/${card.smart_code}`
    : `https://app.comcode.io/c/${card.smart_code}`;

  useEffect(() => {
    QRCode.toDataURL(targetUrl, {
      width: 256,
      margin: 1,
      color: {
        dark: "#09090b",
        light: "#ffffff",
      },
    })
      .then((url) => setQrDataUrl(url))
      .catch((err) => console.error("QR Code generation error:", err));
  }, [targetUrl]);

  return (
    <div className={cn("flex flex-col gap-3 group/card relative", className)}>
      {/* 3D Card Container */}
      <div
        className="relative w-full aspect-[1.586/1] rounded-2xl cursor-pointer perspective-1000 select-none transition-transform duration-300 hover:scale-[1.01]"
        onClick={() => setIsFlipped((prev) => !prev)}
      >
        <div
          className={cn(
            "relative w-full h-full duration-700 transform-style-3d transition-transform rounded-2xl border backdrop-blur-md",
            style.bg,
            style.border,
            isFlipped ? "rotate-y-180" : ""
          )}
        >
          {/* Card Front */}
          <div className="absolute inset-0 w-full h-full p-6 flex flex-col justify-between backface-hidden rounded-2xl overflow-hidden">
            {/* Subtle Texture Highlights */}
            <div className="absolute -top-24 -left-24 w-60 h-60 bg-white/5 rounded-full blur-3xl pointer-events-none" />
            <div className="absolute -bottom-24 -right-24 w-60 h-60 bg-white/5 rounded-full blur-3xl pointer-events-none" />

            {/* Top Bar: Brand, Chip, Contactless Symbol */}
            <div className="flex items-center justify-between relative z-10">
              <div className="flex items-center gap-2">
                <div className="w-7 h-7 rounded-lg bg-white/10 flex items-center justify-center border border-white/20">
                  <Sparkles className="w-4 h-4 text-white" />
                </div>
                <span className="font-bold tracking-wider text-sm text-white font-mono uppercase">
                  ComCode
                </span>
              </div>

              <div className="flex items-center gap-3">
                {card.is_locked && (
                  <Badge variant="danger" className="gap-1 px-2 py-0.5 shadow">
                    <Lock className="w-3 h-3" />
                    {t("cards.fields.is_locked")}
                  </Badge>
                )}
                <div className="flex items-center justify-center w-8 h-8 rounded-full bg-white/5 text-white/80">
                  <Wifi className="w-4 h-4 rotate-90" />
                </div>
              </div>
            </div>

            {/* Middle: Smart Chip & Code */}
            <div className="flex items-center justify-between relative z-10 my-auto">
              <div
                className={cn(
                  "w-11 h-8 rounded-md bg-gradient-to-tr border flex flex-col justify-between p-1 shadow-inner relative overflow-hidden",
                  style.chip
                )}
              >
                <div className="w-full h-0.5 bg-black/20" />
                <div className="w-full h-0.5 bg-black/20" />
                <div className="w-full h-0.5 bg-black/20" />
                <div className="absolute inset-y-0 left-1/3 w-0.5 bg-black/20" />
                <div className="absolute inset-y-0 right-1/3 w-0.5 bg-black/20" />
              </div>
              <span className="font-mono text-xs tracking-widest text-white/50 uppercase">
                {card.smart_code}
              </span>
            </div>

            {/* Bottom: Cardholder Name & Smart Target */}
            <div className="flex items-end justify-between relative z-10">
              <div className="space-y-0.5">
                <h3 className={cn("font-bold text-base tracking-wide truncate max-w-[200px]", style.text)}>
                  {card.name}
                </h3>
                <p className={cn("text-xs flex items-center gap-1.5", style.subtext)}>
                  {card.target_type === "profile" && <User className="w-3 h-3" />}
                  {card.target_type === "direct_url" && <ExternalLink className="w-3 h-3" />}
                  {card.target_type === "vcard_download" && <Share2 className="w-3 h-3" />}
                  <span className="truncate max-w-[180px]">
                    {card.target_type === "profile" && t("cards.settings.target_profile")}
                    {card.target_type === "direct_url" && (card.direct_url || t("cards.target_direct_url_fallback"))}
                    {card.target_type === "vcard_download" && t("cards.target_vcard_label")}
                    {card.target_type === "social_hub" && t("cards.target_social_label")}
                  </span>
                </p>
              </div>

              <div className="flex items-center gap-1.5">
                <Button
                  size="icon"
                  variant="ghost"
                  className="h-8 w-8 rounded-full bg-white/10 text-white hover:bg-white/20"
                  onClick={(e) => {
                    e.stopPropagation();
                    setIsFlipped(true);
                  }}
                  title={t("cards.flip_card")}
                >
                  <RotateCw className="w-3.5 h-3.5" />
                </Button>
              </div>
            </div>
          </div>

          {/* Card Back */}
          <div className="absolute inset-0 w-full h-full p-5 flex flex-col justify-between backface-hidden rotate-y-180 rounded-2xl overflow-hidden">
            {/* Magnetic Stripe Effect */}
            <div className="absolute top-4 inset-x-0 h-10 bg-black/60 shadow-inner" />

            <div className="mt-12 flex items-center justify-between relative z-10">
              {/* QR Code Container */}
              <div className="p-1.5 bg-white rounded-xl shadow-lg border border-black/10">
                {qrDataUrl ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={qrDataUrl}
                    alt="NFC Card QR Code"
                    className="w-20 h-20 rounded-lg object-contain"
                  />
                ) : (
                  <div className="w-20 h-20 bg-zinc-100 flex items-center justify-center rounded-lg">
                    <QrCodeIcon className="w-8 h-8 text-zinc-400 animate-pulse" />
                  </div>
                )}
              </div>

              {/* Back Information */}
              <div className="flex flex-col items-end gap-1.5 text-right font-mono text-xs text-white/80">
                <div className="flex items-center gap-1 text-white">
                  <Wifi className="w-3.5 h-3.5" />
                  <span className="font-semibold text-xs">NFC ENABLED</span>
                </div>
                <span className="text-[10px] text-white/50">{card.card_uid || "TAP TO CONNECT"}</span>
                <span className="text-[10px] text-amber-300/80 font-bold tracking-wider">
                  {card.smart_code}
                </span>
                <span className="text-[10px] text-emerald-400/80 font-medium uppercase">
                  {card.target_type.replace("_", " ")}
                </span>
              </div>
            </div>

            {/* Back Footer */}
            <div className="flex items-center justify-between pt-2 border-t border-white/10 relative z-10 text-[10px] text-white/60">
              <span>Powered by ComCode ID</span>
              <Button
                size="sm"
                variant="ghost"
                className="h-6 text-[10px] px-2 text-white/90 hover:text-white hover:bg-white/10"
                onClick={(e) => {
                  e.stopPropagation();
                  setIsFlipped(false);
                }}
              >
                {t("cards.flip_front")}
              </Button>
            </div>
          </div>
        </div>
      </div>

      {/* Action Toolbar Below Card */}
      <div className="flex items-center justify-between px-1">
        <div className="flex items-center gap-1.5">
          <Badge variant={card.is_active ? "success" : "secondary"} className="text-xs">
            {card.is_active ? t("common.active") : t("common.inactive")}
          </Badge>
          <span className="text-xs text-muted-foreground">
            {materialLabel}
          </span>
        </div>

        <div className="flex items-center gap-1">
          <Button
            size="sm"
            variant="outline"
            className="h-8 gap-1.5 text-xs"
            onClick={onQRClick}
          >
            <QrCodeIcon className="w-3.5 h-3.5" />
            QR
          </Button>
          <Button
            size="sm"
            variant="outline"
            className={cn(
              "h-8 gap-1.5 text-xs",
              card.is_locked ? "text-amber-500 border-amber-500/40 hover:bg-amber-500/10" : ""
            )}
            onClick={onToggleLock}
          >
            <Lock className="w-3.5 h-3.5" />
            {card.is_locked ? t("cards.unlock") : t("cards.lock")}
          </Button>
          <Button
            size="sm"
            variant="default"
            className="h-8 gap-1.5 text-xs"
            onClick={onSettingsClick}
          >
            <Settings className="w-3.5 h-3.5" />
            {t("common.edit")}
          </Button>
        </div>
      </div>
    </div>
  );
}
