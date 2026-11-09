import type { components } from "@/generated/api";

export type PublicUser = components["schemas"]["PublicUser"];
export type LoginRequest = components["schemas"]["LoginRequest"];
export type Me = components["schemas"]["Me"];
export type Tokens = components["schemas"]["Tokens"];

export type RoleSummary = {
  uuid: string;
  name: string;
  slug: string;
  description?: string | null;
  is_system?: boolean;
};

/** An account the user belongs to (docs/TENANCY_AND_ACCESS.md). */
export type AccountSummary = {
  uuid: string;
  type: "personal" | "company";
  name: string;
  slug: string;
  status: string;
  role: "owner" | "admin" | "office_manager" | "member";
  active: boolean;
};

/** Client session identity — profile + RBAC + active account. */
export type AuthUser = {
  uuid: string;
  email: string;
  name: string;
  surname: string;
  fullName: string;
  status: string;
  locale: string;
  isSuperAdmin: boolean;
  emailVerified: boolean;
  permissions: string[];
  roles: string[];
  realtimeUserChannel?: string;
  realtimeEnabled?: boolean;
  /** Account this session acts in; app data is scoped to it. */
  account?: AccountSummary;
  accounts: AccountSummary[];
  impersonation?: {
    uuid: string;
    email: string;
    name: string;
    surname: string;
    fullName: string;
  };
};

export function mapMeToAuthUser(me: Me): AuthUser {
  const { user, roles, permissions, realtime, impersonation, account, accounts } = me;
  const locale =
    typeof user.locale === "string" && user.locale ? user.locale : "tr";
  const mapped: AuthUser = {
    uuid: user.uuid,
    email: user.email,
    name: user.name,
    surname: user.surname,
    fullName: `${user.name} ${user.surname}`.trim(),
    status: user.status,
    locale,
    isSuperAdmin: Boolean(user.is_super_admin),
    emailVerified: Boolean(user.email_verified),
    permissions: permissions ?? [],
    roles: roles ?? [],
    realtimeUserChannel: realtime?.user_channel,
    realtimeEnabled: realtime?.enabled,
    account: account ? (account as AccountSummary) : undefined,
    accounts: (accounts ?? []) as AccountSummary[],
  };
  if (impersonation?.user) {
    const actor = impersonation.user;
    mapped.impersonation = {
      uuid: actor.uuid,
      email: actor.email,
      name: actor.name,
      surname: actor.surname,
      fullName: `${actor.name} ${actor.surname}`.trim(),
    };
  }
  return mapped;
}

export function hasPlatformPermission(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  if (user.isSuperAdmin || user.roles.includes("super_admin")) return true;
  return user.permissions.some((p) => p.startsWith("platform."));
}

export function isPlatformUser(user: AuthUser | null | undefined): boolean {
  return hasPlatformPermission(user);
}

export function isSubscriber(user: AuthUser | null | undefined): boolean {
  if (!user) return false;
  return !isPlatformUser(user);
}

/** @deprecated Use isSubscriber */
export const isCmsUser = isSubscriber;

export function defaultHomeForUser(user: AuthUser): string {
  if (isPlatformUser(user)) return "/platform";
  return "/";
}

/** Owners and admins may change the active account's resources and members. */
export function canManageAccount(user: AuthUser | null | undefined): boolean {
  const role = user?.account?.role;
  return role === "owner" || role === "admin";
}
