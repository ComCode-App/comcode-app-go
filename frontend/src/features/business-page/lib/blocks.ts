import { createElement, type ReactElement } from "react";
import {
  Briefcase,
  Building2,
  Calendar,
  Download,
  FileText,
  Globe,
  Home,
  Landmark,
  Link as LinkIcon,
  Mail,
  MapPin,
  MessageCircle,
  Phone,
  ShieldCheck,
  ShoppingBag,
  Star,
  type LucideIcon,
} from "lucide-react";

import type {
  BusinessBlockType,
  BusinessButtonTarget,
} from "../services/business-page.service";

/** Block types in the "add block" menu order (TEC-1327). */
export const BLOCK_TYPES: BusinessBlockType[] = [
  "header",
  "button",
  "social",
  "offices",
  "team",
  "bank_accounts",
];

export const BUTTON_TARGETS: BusinessButtonTarget[] = [
  "url",
  "phone",
  "whatsapp",
  "email",
  "maps",
  "google_review",
  "lead_form",
];

/** Icons a button may pick; "" uses the target's default. */
export const BUTTON_ICONS: Record<string, LucideIcon> = {
  link: LinkIcon,
  globe: Globe,
  phone: Phone,
  whatsapp: MessageCircle,
  mail: Mail,
  map: MapPin,
  star: Star,
  form: FileText,
  shop: ShoppingBag,
  home: Home,
  briefcase: Briefcase,
  building: Building2,
  bank: Landmark,
  shield: ShieldCheck,
  calendar: Calendar,
  download: Download,
};

const TARGET_ICON: Record<BusinessButtonTarget, string> = {
  url: "link",
  phone: "phone",
  whatsapp: "whatsapp",
  email: "mail",
  maps: "map",
  google_review: "star",
  lead_form: "form",
};

/** Icon of a button: its own pick, else the target's default. */
export function buttonIcon(target: string, icon?: string): LucideIcon {
  return (
    BUTTON_ICONS[icon || ""] ??
    BUTTON_ICONS[TARGET_ICON[target as BusinessButtonTarget] ?? "link"] ??
    LinkIcon
  );
}

/** The button icon as an element (rendered from the static icon map, not a component made in render). */
export function buttonIconElement(
  target: string,
  icon: string | undefined,
  className: string,
): ReactElement {
  return createElement(buttonIcon(target, icon), {
    className,
    "aria-hidden": true,
  });
}

const digits = (v: string) => v.replace(/[^\d+]/g, "");

/** Where a button goes; null for the lead form (opened on the page) or an empty value. */
export function buttonHref(target: string, value: string): string | null {
  const v = value.trim();
  switch (target) {
    case "url":
    case "google_review":
      return /^https?:\/\//i.test(v) ? v : null;
    case "phone":
      return v ? `tel:${digits(v)}` : null;
    case "whatsapp":
      return v ? `https://wa.me/${digits(v).replace(/^\+/, "")}` : null;
    case "email":
      return v ? `mailto:${v}` : null;
    case "maps":
      if (/^https?:\/\//i.test(v)) return v;
      return v
        ? `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(v)}`
        : null;
    default:
      return null;
  }
}

/** "Get directions" link for an office address. */
export function officeMapsHref(parts: string[]): string {
  return `https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(
    parts.filter(Boolean).join(", "),
  )}`;
}

/** Same shape rule as backend publicpath (3-40 chars, a-z 0-9, single . _ - between). */
export function isUsernameShape(value: string): boolean {
  return (
    value.length >= 3 &&
    value.length <= 40 &&
    /^[a-z0-9](?:[a-z0-9]|[._-][a-z0-9])*$/.test(value)
  );
}

const FOLD: Record<string, string> = {
  ç: "c",
  ğ: "g",
  ı: "i",
  İ: "i",
  ö: "o",
  ş: "s",
  ü: "u",
};

/** A username suggestion from the company name ("Swiss Garage" -> "swiss-garage"). */
export function suggestUsername(name: string): string {
  return name
    .toLocaleLowerCase("tr")
    .replace(/[çğıİöşü]/g, (c) => FOLD[c] ?? c)
    .normalize("NFKD")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 40)
    .replace(/-+$/g, "");
}
