import NextAuth, { CredentialsSignin } from "next-auth";
import type { Provider } from "next-auth/providers";
import Apple from "next-auth/providers/apple";
import Credentials from "next-auth/providers/credentials";
import Facebook from "next-auth/providers/facebook";
import GitHub from "next-auth/providers/github";
import Google from "next-auth/providers/google";
import Passkey from "next-auth/providers/passkey";
import { NextResponse } from "next/server";

import { routes } from "@/config/routes";
import { goAdapter } from "@/lib/auth/go-adapter";
import {
  adapterGetOAuthConfig,
  adapterGetOAuthAccountUser,
  adapterGetUserByEmail,
  adapterIssueSession,
  loginWithPassword,
  type OAuthConfigPayload,
} from "@/lib/auth/go-adapter-client";
import { appleFormPostCookies } from "@/lib/auth/apple-form-post";
import { createProviderCache } from "@/lib/auth/provider-cache";
import { secureAuthCookies } from "@/lib/auth/secure-cookies";
import {
  logAuthError,
  logCredentialsFailure,
} from "@/lib/observability/auth-logger";
import { fetchAppPublicConfig } from "@/services/app-config.service";

const pendingGitHubLogins = new Map<string, string>();

class MFARequiredError extends CredentialsSignin {
  code = "MFA_REQUIRED";
}

class InvalidMFACodeError extends CredentialsSignin {
  code = "INVALID_MFA_CODE";
}

class MFANotEnrolledError extends CredentialsSignin {
  code = "MFA_NOT_ENROLLED";
}

const OAUTH_PROVIDERS = ["github", "google", "facebook", "apple"] as const;
type OAuthProviderId = (typeof OAUTH_PROVIDERS)[number];

const GUEST_AUTH_PREFIXES = [
  routes.guest.login,
  routes.guest.register,
  routes.guest.forgotPassword,
  routes.guest.resetPassword,
  routes.guest.verifyEmail,
  routes.guest.inviteRoot,
];

function isGuestAuthPath(pathname: string) {
  return GUEST_AUTH_PREFIXES.some(
    (path) => pathname === path || pathname.startsWith(`${path}/`),
  );
}

function isProtectedPath(pathname: string) {
  if (pathname === routes.app.root) return true;
  if (pathname.startsWith(`${routes.app.profile.root}`)) return true;
  if (pathname === routes.platform.root) return true;
  if (
    pathname.startsWith(`${routes.platform.root}/`) &&
    !isGuestAuthPath(pathname)
  ) {
    return true;
  }
  return false;
}

function isOAuthProvider(
  provider: string | undefined,
): provider is OAuthProviderId {
  return (
    provider !== undefined &&
    (OAUTH_PROVIDERS as readonly string[]).includes(provider)
  );
}

type OAuthConfigResult = {
  config: OAuthConfigPayload | null;
  /** False when the backend call failed (the provider then stays off for this build). */
  ok: boolean;
  /** Epoch ms after which the providers must be rebuilt (generated client secret). */
  usableUntil?: number;
};

/**
 * A generated client secret (Apple signing key, valid 24 h) is never handed to
 * Auth.js this close to its expiry, and the provider cache is rebuilt before it.
 */
const SECRET_EXPIRY_MARGIN_MS = 10 * 60 * 1000;

async function loadOAuthConfig(
  provider: OAuthProviderId,
): Promise<OAuthConfigResult> {
  try {
    const config = await adapterGetOAuthConfig(provider);
    if (config.enabled && config.client_id && config.client_secret) {
      if (config.client_secret_expires_at) {
        const exp = Date.parse(config.client_secret_expires_at);
        if (Number.isFinite(exp)) {
          const usableUntil = exp - SECRET_EXPIRY_MARGIN_MS;
          // Already (nearly) expired: keep the provider off and retry next build.
          if (usableUntil <= Date.now()) return { config: null, ok: false };
          return { config, ok: true, usableUntil };
        }
      }
      return { config, ok: true };
    }
    return { config: null, ok: true };
  } catch {
    // Provider stays disabled when config is unavailable.
    return { config: null, ok: false };
  }
}

async function loadLoginMethods() {
  try {
    const appConfig = await fetchAppPublicConfig("en");
    return {
      passwordLogin: appConfig.auth_methods.password.login,
      passkeyLogin: appConfig.auth_methods.passkey.login,
      ok: true,
    };
  } catch {
    // Keep defaults when public config is unavailable.
    return { passwordLogin: true, passkeyLogin: true, ok: false };
  }
}

async function buildProviders(): Promise<{
  value: Provider[];
  complete: boolean;
  expiresAt: number | null;
}> {
  // All backend lookups run in parallel (TEC-1113).
  const [methods, githubResult, googleResult, facebookResult, appleResult] =
    await Promise.all([
      loadLoginMethods(),
      loadOAuthConfig("github"),
      loadOAuthConfig("google"),
      loadOAuthConfig("facebook"),
      loadOAuthConfig("apple"),
    ]);
  const { passwordLogin, passkeyLogin } = methods;
  const complete = [
    methods,
    githubResult,
    googleResult,
    facebookResult,
    appleResult,
  ].every((r) => r.ok);
  const secretDeadlines = [
    githubResult,
    googleResult,
    facebookResult,
    appleResult,
  ].flatMap((r) => (r.usableUntil ? [r.usableUntil] : []));
  const expiresAt = secretDeadlines.length
    ? Math.min(...secretDeadlines)
    : null;

  const providers: Provider[] = [];

  if (passwordLogin) {
    providers.push(
      Credentials({
        credentials: {
          email: { label: "Email", type: "email" },
          password: { label: "Password", type: "password" },
          totp_code: { label: "Authenticator code", type: "text" },
        },
        async authorize(credentials) {
          const email = String(credentials?.email ?? "").trim();
          const password = String(credentials?.password ?? "");
          const totpCode = String(credentials?.totp_code ?? "").trim();
          if (!email || !password) {
            console.warn("[auth][credentials] missing email or password", {
              hasEmail: Boolean(email),
              hasPassword: Boolean(password),
            });
            return null;
          }

          try {
            const tokens = await loginWithPassword(
              email,
              password,
              totpCode || undefined,
            );
            const user = await adapterGetUserByEmail(email);
            return {
              id: user.id,
              email: user.email,
              name: user.name,
              accessToken: tokens.access_token,
              refreshToken: tokens.refresh_token,
              expiresIn: tokens.expires_in,
            };
          } catch (error) {
            const code = (error as Error & { code?: string }).code;
            // Code + status only: never the email, password or authenticator code.
            logCredentialsFailure(error);
            if (code === "MFA_REQUIRED") {
              throw new MFARequiredError();
            }
            if (code === "INVALID_MFA_CODE") {
              throw new InvalidMFACodeError();
            }
            if (code === "MFA_NOT_ENROLLED") {
              throw new MFANotEnrolledError();
            }
            return null;
          }
        },
      }),
    );
  }

  if (passkeyLogin) {
    providers.push(Passkey({}));
  }

  const github = githubResult.config;
  if (github) {
    providers.push(
      GitHub({
        clientId: github.client_id,
        clientSecret: github.client_secret,
      }),
    );
  }

  const google = googleResult.config;
  if (google) {
    providers.push(
      Google({
        clientId: google.client_id,
        clientSecret: google.client_secret,
      }),
    );
  }

  const facebook = facebookResult.config;
  if (facebook) {
    providers.push(
      Facebook({
        clientId: facebook.client_id,
        clientSecret: facebook.client_secret,
      }),
    );
  }

  const apple = appleResult.config;
  if (apple) {
    providers.push(
      Apple({
        clientId: apple.client_id,
        clientSecret: apple.client_secret,
      }),
    );
  }

  return { value: providers, complete, expiresAt };
}

/** Shared across requests in this server process; TTL in provider-cache.ts. */
const providerCache = createProviderCache(buildProviders);

function requestPathname(request: Request | undefined): string {
  if (!request) return "";
  try {
    return new URL(request.url).pathname;
  } catch {
    return "";
  }
}

const authHandlers = NextAuth(async (request) => {
  // Apple's form_post callback is a cross-site POST: its check cookies are
  // SameSite=None on the Apple sign-in / callback requests only (TEC-1123).
  const appleCookies = appleFormPostCookies(
    requestPathname(request),
    secureAuthCookies(),
  );
  return {
    adapter: goAdapter({ pendingGitHubLogins }),
    trustHost: process.env.AUTH_TRUST_HOST === "true",
    useSecureCookies: secureAuthCookies(),
    ...(appleCookies ? { cookies: appleCookies } : {}),
    session: { strategy: "jwt" },
    logger: { error: logAuthError },
    providers: await providerCache.get(),
    experimental: {
      enableWebAuthn: true,
    },
    pages: {
      signIn: routes.guest.login,
      error: routes.guest.login,
    },
    callbacks: {
      authorized({ auth, request }) {
        const { pathname } = request.nextUrl;
        if (!isProtectedPath(pathname) || auth) {
          return true;
        }

        const url = request.nextUrl.clone();
        url.pathname = routes.guest.login;
        // Keep the query so the deep link survives sign-in; the login form runs
        // it through safeNextPath before redirecting.
        url.search = "";
        url.searchParams.set("next", `${pathname}${request.nextUrl.search}`);
        return NextResponse.redirect(url);
      },
      async signIn({ account, profile }) {
        if (!isOAuthProvider(account?.provider)) {
          return true;
        }

        const provider = account.provider;

        if (
          provider === "github" &&
          profile &&
          typeof profile === "object" &&
          "login" in profile &&
          typeof profile.login === "string" &&
          account.providerAccountId
        ) {
          pendingGitHubLogins.set(account.providerAccountId, profile.login);
        }

        const providerAccountId = account.providerAccountId ?? "";
        try {
          await adapterGetOAuthAccountUser(provider, providerAccountId);
          return true;
        } catch {
          // Not linked yet.
        }

        const { getApiTokens } = await import("@/lib/server/auth-tokens");
        const session = await getApiTokens();
        if (session.userId) {
          return true;
        }

        try {
          const config = await adapterGetOAuthConfig(provider);
          if (config.register_allowed) {
            return true;
          }
        } catch {
          // fall through
        }

        return `${routes.guest.login}?error=OAuthNotLinked`;
      },
      async jwt({ token, user, account, trigger, session }) {
        if (user) {
          token.sub = user.id;
          token.email = user.email;
          const extended = user as typeof user & {
            accessToken?: string;
            refreshToken?: string;
            expiresIn?: number;
          };
          if (extended.accessToken && extended.refreshToken) {
            token.accessToken = extended.accessToken;
            token.refreshToken = extended.refreshToken;
            token.expiresIn = extended.expiresIn;
          }
        }

        if (
          account &&
          (account.provider === "passkey" ||
            isOAuthProvider(account.provider)) &&
          token.sub &&
          !token.accessToken
        ) {
          try {
            const authMethod =
              account.provider === "passkey" ? "passkey" : "oauth";
            const tokens = await adapterIssueSession(token.sub, authMethod);
            token.accessToken = tokens.access_token;
            token.refreshToken = tokens.refresh_token;
            token.expiresIn = tokens.expires_in;
          } catch {
            token.error = isOAuthProvider(account.provider)
              ? "OAuthSessionError"
              : "PasskeySessionError";
          }
        }

        if (trigger === "update" && session) {
          const patch = session as {
            accessToken?: string;
            refreshToken?: string;
            expiresIn?: number;
          };
          if (patch.accessToken) token.accessToken = patch.accessToken;
          if (patch.refreshToken) token.refreshToken = patch.refreshToken;
          if (patch.expiresIn) token.expiresIn = patch.expiresIn;
        }

        return token;
      },
      async session({ session, token }) {
        if (session.user) {
          session.user.id = token.sub ?? "";
        }
        (session as { error?: string }).error = token.error as
          "PasskeySessionError" | "OAuthSessionError" | undefined;
        return session;
      },
    },
  };
});

export const { handlers, auth, signIn, signOut } = authHandlers;
