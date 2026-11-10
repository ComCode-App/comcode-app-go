import type { components } from "@/generated/api";
import { apiClient, unwrap } from "@/lib/api";
import type { Me } from "@/lib/auth/types";

type AuthSessionResult = {
  authenticated?: boolean;
  expires_in?: number;
};

export type PasskeySummary = {
  uuid: string;
  name?: string | null;
  device_type: string;
  backed_up: boolean;
  transports?: string | null;
  last_used_at?: string | null;
  created_at: string;
};

export type LinkedIdentitySummary = {
  provider: string;
  github_login?: string | null;
  linked_at: string;
};

type PasskeyListResult = {
  items: PasskeySummary[];
  total: number;
};

type IdentityListResult = {
  items: LinkedIdentitySummary[];
  total: number;
  /** False when the user never chose a password (OAuth sign-up). */
  has_password: boolean;
  has_passkey: boolean;
};

export type DeviceSession = {
  uuid: string;
  current: boolean;
  user_agent?: string | null;
  ip_address?: string | null;
  created_at: string;
  expires_at: string;
  impersonated: boolean;
  /** "web" or "mobile" (TEC-1179). */
  client_type?: "web" | "mobile";
  device_name?: string | null;
  app_version?: string | null;
};

type SessionListResult = {
  items: DeviceSession[];
};

type StatusPayload = components["schemas"]["StatusPayload"];
export type QRLoginCreated = components["schemas"]["QRLoginCreated"];
export type QRLoginState = components["schemas"]["QRLoginState"];
type PatchProfileRequest = components["schemas"]["PatchProfileRequest"];
type ForgotPasswordRequest = components["schemas"]["ForgotPasswordRequest"];
type ResetPasswordRequest = components["schemas"]["ResetPasswordRequest"];
type ChangePasswordRequest = components["schemas"]["ChangePasswordRequest"];
type VerifyEmailRequest = components["schemas"]["VerifyEmailRequest"];
type RegisterRequest = components["schemas"]["RegisterRequest"];

/**
 * Thin OpenAPI wrappers — Go API tokens live in the NextAuth JWT (server-only).
 * BFF reads tokens from the session cookie when proxying upstream.
 */
export const authService = {
  async register(body: RegisterRequest) {
    return unwrap<{ user: Me }>(
      await apiClient.POST("/v1/auth/register", { body }),
    );
  },

  async refresh() {
    return unwrap<AuthSessionResult>(
      await apiClient.POST("/v1/auth/refresh", {
        body: { refresh_token: "" },
      }),
    );
  },

  async logout() {
    try {
      await unwrap(
        await apiClient.POST("/v1/auth/logout", {
          body: { refresh_token: "" },
        }),
        { silent: true },
      );
    } catch {
      // Cookie clear still happens server-side on logout route
    }
  },

  async getMe() {
    return unwrap<Me>(await apiClient.GET("/v1/auth/me"));
  },

  async updateProfile(body: PatchProfileRequest) {
    return unwrap<Me>(await apiClient.PATCH("/v1/auth/profile", { body }));
  },

  async changePassword(body: ChangePasswordRequest) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/password/change", { body }),
    );
  },

  /** Starts a QR sign-in for this login tab (anonymous; TEC-1185). */
  async createQRLoginSession() {
    return unwrap<QRLoginCreated>(
      await apiClient.POST("/v1/auth/qr/sessions"),
      { silent: true },
    );
  },

  /** One catch-up read after the tab (re)subscribes to its channel; never polled. */
  async getQRLoginState(sessionId: string, browserSecret: string) {
    return unwrap<QRLoginState>(
      await apiClient.POST("/v1/auth/qr/sessions/{id}/state", {
        params: { path: { id: sessionId } },
        body: { browser_secret: browserSecret },
      }),
      { silent: true },
    );
  },

  /** Emails a sign-in (or sign-up) code; the answer is the same for every email (TEC-1121). */
  async requestEmailCode(email: string) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/email-code/request", {
        body: { email },
      }),
      { silent: true },
    );
  },

  async forgotPassword(body: ForgotPasswordRequest) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/password/forgot", { body }),
    );
  },

  async resetPassword(body: ResetPasswordRequest) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/password/reset", { body }),
    );
  },

  async requestEmailVerify(email?: string) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/email/verify-request", {
        body: email ? { email } : {},
      }),
    );
  },

  async verifyEmail(body: VerifyEmailRequest) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/email/verify", { body }),
    );
  },

  async listPasskeys() {
    return unwrap<PasskeyListResult>(await apiClient.GET("/v1/auth/passkeys"));
  },

  async renamePasskey(uuid: string, name: string | null) {
    return unwrap<PasskeySummary>(
      await apiClient.PATCH("/v1/auth/passkeys/{uuid}", {
        params: { path: { uuid } },
        body: { name },
      }),
    );
  },

  async deletePasskey(uuid: string) {
    return unwrap<StatusPayload>(
      await apiClient.DELETE("/v1/auth/passkeys/{uuid}", {
        params: { path: { uuid } },
      }),
    );
  },

  async listIdentities() {
    return unwrap<IdentityListResult>(
      await apiClient.GET("/v1/auth/identities"),
    );
  },

  /** Switches the session's active account; the BFF stores the returned tokens. */
  async switchAccount(accountUuid: string) {
    return unwrap<{ account: { uuid: string; name: string } }>(
      await apiClient.POST("/v1/auth/accounts/switch", {
        body: { account_uuid: accountUuid },
      }),
    );
  },

  async stopImpersonation() {
    return unwrap<{ session: { user_uuid: string; email: string } }>(
      await apiClient.POST("/v1/auth/impersonation/stop"),
    );
  },

  async unlinkIdentity(provider: "github" | "google" | "facebook" | "apple") {
    return unwrap<StatusPayload>(
      await apiClient.DELETE("/v1/auth/identities/{provider}", {
        params: { path: { provider } },
      }),
    );
  },

  async unlinkGitHub() {
    return this.unlinkIdentity("github");
  },

  async getTOTPStatus() {
    return unwrap<components["schemas"]["TOTPStatus"]>(
      await apiClient.GET("/v1/auth/totp"),
    );
  },

  async setupTOTP() {
    return unwrap<components["schemas"]["TOTPSetup"]>(
      await apiClient.POST("/v1/auth/totp/setup"),
    );
  },

  async confirmTOTP(code: string) {
    return unwrap<components["schemas"]["TOTPConfirm"]>(
      await apiClient.POST("/v1/auth/totp/confirm", { body: { code } }),
    );
  },

  async disableTOTP(code: string) {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/totp/disable", { body: { code } }),
    );
  },

  async listSessions() {
    return unwrap<SessionListResult>(await apiClient.GET("/v1/auth/sessions"));
  },

  async revokeSession(uuid: string) {
    return unwrap<StatusPayload>(
      await apiClient.DELETE("/v1/auth/sessions/{uuid}", {
        params: { path: { uuid } },
      }),
    );
  },

  async revokeOtherSessions() {
    return unwrap<StatusPayload>(
      await apiClient.POST("/v1/auth/sessions/revoke-others"),
    );
  },
};
