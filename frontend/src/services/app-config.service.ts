import type { AppLocale } from "@/config/i18n";
import { apiConfig } from "@/config/api";

export type AuthMethodFlags = {
  login: boolean;
  register?: boolean;
};

export type AppPublicConfig = {
  mode: string;
  name: string;
  vapid_configured: boolean;
  github_auth_enabled: boolean;
  registration_enabled: boolean;
  auth_methods: {
    password: AuthMethodFlags;
    passkey: { login: boolean };
    github: AuthMethodFlags;
    google: AuthMethodFlags;
    facebook: AuthMethodFlags;
    apple: AuthMethodFlags;
  };
  /** Store pages of the mobile app (TEC-1178); "" when not set. */
  mobile_store_urls: { ios: string; android: string };
};

const defaultMethods = (): AppPublicConfig["auth_methods"] => ({
  password: { login: true, register: false },
  passkey: { login: true },
  github: { login: false, register: false },
  google: { login: false, register: false },
  facebook: { login: false, register: false },
  apple: { login: false, register: false },
});

export async function fetchAppPublicConfig(
  locale: AppLocale = "en",
): Promise<AppPublicConfig> {
  const response = await fetch(`${apiConfig.baseUrl}/v1/app/config`, {
    headers: {
      Accept: "application/json",
      "Accept-Language": locale,
    },
    cache: "no-store",
  });

  if (!response.ok) {
    throw new Error("Failed to load app config");
  }

  const json = (await response.json()) as {
    success?: boolean;
    data?: Partial<AppPublicConfig> & {
      mobile?: { store_urls?: { ios?: string; android?: string } };
    };
  };

  if (!json.data) {
    throw new Error("Invalid app config response");
  }

  const methods = {
    ...defaultMethods(),
    ...(json.data.auth_methods ?? {}),
  };

  return {
    mode: json.data.mode ?? "platform",
    name: json.data.name ?? "",
    vapid_configured: Boolean(json.data.vapid_configured),
    github_auth_enabled:
      methods.github.login || Boolean(json.data.github_auth_enabled),
    registration_enabled: Boolean(json.data.registration_enabled),
    auth_methods: methods,
    mobile_store_urls: {
      ios: httpsUrlOrEmpty(json.data.mobile?.store_urls?.ios),
      android: httpsUrlOrEmpty(json.data.mobile?.store_urls?.android),
    },
  };
}

/** Store links are rendered as `href`: only https URLs pass. */
function httpsUrlOrEmpty(raw: string | undefined): string {
  if (!raw) return "";
  try {
    return new URL(raw).protocol === "https:" ? raw : "";
  } catch {
    return "";
  }
}
