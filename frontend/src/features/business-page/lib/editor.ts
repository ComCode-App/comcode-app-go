import type {
  BusinessBlockText,
  BusinessBlockType,
  BusinessPage,
  BusinessPageText,
  SaveBusinessPageRequest,
} from "../services/business-page.service";

export type Locale = "tr" | "en";
export const LOCALES: Locale[] = ["tr", "en"];

export type HeaderConfig = {
  logo_media_uuid: string | null;
  cover_media_uuid: string | null;
};
export type ButtonConfig = { target: string; value: string; icon: string };
export type SocialConfig = { links: { platform: string; url: string }[] };
export type BankAccountsConfig = { bank_account_uuids: string[] };

export type EditorBlock = {
  /** Local key for sorting and React; not sent. */
  key: string;
  type: BusinessBlockType;
  enabled: boolean;
  config: Record<string, unknown>;
  translations: Partial<Record<Locale, BusinessBlockText>>;
};

export type EditorState = {
  username: string;
  theme: string;
  primaryLocale: Locale;
  translations: Partial<Record<Locale, BusinessPageText>>;
  blocks: EditorBlock[];
};

let seq = 0;
export const newKey = () =>
  `b${Date.now().toString(36)}${(seq++).toString(36)}`;

export function defaultConfig(
  type: BusinessBlockType,
): Record<string, unknown> {
  switch (type) {
    case "header":
      return { logo_media_uuid: null, cover_media_uuid: null };
    case "button":
      return { target: "url", value: "", icon: "" };
    case "social":
      return { links: [] };
    case "bank_accounts":
      return { bank_account_uuids: [] };
    default:
      return {};
  }
}

export function newBlock(type: BusinessBlockType): EditorBlock {
  return {
    key: newKey(),
    type,
    enabled: true,
    config: defaultConfig(type),
    translations: {},
  };
}

/** Starting point of a company without a page: a header and a website button. */
export function emptyState(name: string, username: string): EditorState {
  return {
    username,
    theme: "",
    primaryLocale: "tr",
    translations: { tr: { name, tagline: "" } },
    blocks: [newBlock("header"), newBlock("button")],
  };
}

export function fromPage(page: BusinessPage): EditorState {
  return {
    username: page.username,
    theme: page.theme,
    primaryLocale: page.primary_locale,
    translations: page.translations as EditorState["translations"],
    blocks: page.blocks.map((b) => ({
      key: newKey(),
      type: b.type,
      enabled: b.enabled,
      config: { ...defaultConfig(b.type), ...(b.config ?? {}) },
      translations: b.translations as EditorBlock["translations"],
    })),
  };
}

/** Drops empty locale entries so the API stores only what was written. */
function compact<T extends object>(
  m: Partial<Record<Locale, T>>,
): Record<string, T> {
  const out: Record<string, T> = {};
  for (const [k, v] of Object.entries(m)) {
    if (v && Object.values(v).some((x) => String(x ?? "").trim() !== ""))
      out[k] = v as T;
  }
  return out;
}

/** What is sent for a block: a lead form button has no value, half-filled social rows are left out. */
function cleanConfig(b: EditorBlock): Record<string, unknown> {
  if (b.type === "button" && (b.config as ButtonConfig).target === "lead_form")
    return { ...b.config, value: "" };
  if (b.type === "social")
    return {
      links: (b.config as SocialConfig).links
        .map((l) => ({ platform: l.platform, url: l.url.trim() }))
        .filter((l) => l.platform && l.url),
    };
  return b.config;
}

export function toRequest(state: EditorState): SaveBusinessPageRequest {
  return {
    username: state.username.trim().toLowerCase(),
    theme: state.theme,
    primary_locale: state.primaryLocale,
    translations: compact(state.translations),
    blocks: state.blocks.map((b) => ({
      type: b.type,
      enabled: b.enabled,
      config: cleanConfig(b),
      translations: compact(b.translations),
    })),
  };
}
