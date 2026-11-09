import crypto from "node:crypto";

import { apiURL } from "./env";

type Envelope<T> = {
  success: boolean;
  data: T;
  error?: { code?: string; message?: string };
};

// Each setup run uses its own forwarded IP so the per-IP auth rate limits
// (register/login, 15 min windows) never block repeated local runs.
// The API honours X-Forwarded-For only from TRUSTED_PROXIES (loopback by default).
const forwardedFor = `10.${rand()}.${rand()}.${rand()}`;

function rand() {
  return crypto.randomInt(1, 254);
}

export async function api<T>(
  method: string,
  path: string,
  opts: { token?: string; body?: unknown } = {},
): Promise<T> {
  const res = await fetch(`${apiURL}${path}`, {
    method,
    headers: {
      "content-type": "application/json",
      "x-forwarded-for": forwardedFor,
      ...(opts.token ? { authorization: `Bearer ${opts.token}` } : {}),
    },
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(`${method} ${path} → ${res.status}: ${text}`);
  }
  return (text ? (JSON.parse(text) as Envelope<T>).data : undefined) as T;
}

export async function login(email: string, password: string): Promise<string> {
  const data = await api<{ access_token: string }>("POST", "/auth/login", {
    body: { email, password },
  });
  return data.access_token;
}

export async function roleUUID(token: string, slug: string): Promise<string> {
  const data = await api<{ items: { uuid: string; slug: string }[] }>(
    "GET",
    "/platform/roles?limit=100",
    { token },
  );
  const role = data.items.find((r) => r.slug === slug);
  if (!role) throw new Error(`role ${slug} not found`);
  return role.uuid;
}

export async function createUser(
  adminToken: string,
  user: { email: string; password: string; name: string; surname: string },
  roleUUIDs: string[],
) {
  await api("POST", "/platform/users", {
    token: adminToken,
    body: { ...user, status: "active", role_uuids: roleUUIDs },
  });
}

/** Saves the user's UI language; after sign-in it overrides the browser's choice. */
export async function setLocale(token: string, locale: string) {
  await api("PATCH", "/auth/profile", { token, body: { locale } });
}

/** Enrols TOTP for the user and returns the base32 secret. */
export async function enableTotp(
  token: string,
  code: (secret: string) => string,
): Promise<string> {
  const setup = await api<{ secret: string }>("POST", "/auth/totp/setup", {
    token,
  });
  await api("POST", "/auth/totp/confirm", {
    token,
    body: { code: code(setup.secret) },
  });
  return setup.secret;
}

export type ProfileSeed = {
  firstName: string;
  lastName: string;
  company?: string;
  title?: string;
  email?: string;
};

/** Creates a profile with one translation and returns its uuid. */
export async function createProfile(
  token: string,
  seed: ProfileSeed,
): Promise<string> {
  const profile = await api<{ uuid: string }>("POST", "/profiles", {
    token,
    body: {
      primary_locale: "tr",
      is_primary: true,
      translations: [
        {
          locale: "tr",
          first_name: seed.firstName,
          last_name: seed.lastName,
          company_name: seed.company ?? "",
          title: seed.title ?? "",
          is_default: true,
        },
      ],
      contact_details: seed.email
        ? [
            {
              category: "email",
              type: "work",
              value: seed.email,
              is_primary: true,
            },
          ]
        : [],
    },
  });
  return profile.uuid;
}

/** Updates a profile's public page and returns its username. */
export async function updatePublicPage(
  token: string,
  profileUuid: string,
  body: Record<string, unknown>,
): Promise<string> {
  const page = await api<{ username: string }>(
    "PUT",
    `/profiles/${profileUuid}/public-page`,
    { token, body },
  );
  return page.username;
}
