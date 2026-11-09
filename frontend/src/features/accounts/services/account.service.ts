import { platformRequest } from "@/lib/api/platform-request";

export type AccountRole = "owner" | "admin" | "office_manager" | "member";

export type ActiveAccount = {
  uuid: string;
  type: "personal" | "company";
  name: string;
  slug: string;
  status: string;
  role: AccountRole;
  member_count: number;
};

export type AccountMember = {
  user_uuid: string;
  email: string;
  name: string;
  surname: string;
  role: AccountRole;
  created_at: string;
};

export type InviteStatus = "pending" | "expired" | "accepted" | "revoked";

export type AccountInvite = {
  uuid: string;
  email: string;
  role: AccountRole;
  status: InviteStatus;
  source: "manual" | "import";
  expires_at: string;
  last_sent_at?: string;
  send_count: number;
  profile_uuid: string | null;
  invited_by_name: string;
  created_at: string;
};

export type InvitePreview = {
  account_name: string;
  email: string;
  role: AccountRole;
  status: InviteStatus;
  expires_at: string;
  invited_by_name: string;
  user_exists: boolean;
};

export type InviteAcceptResult = {
  account_uuid: string;
  account_name: string;
  role: AccountRole;
  profile_uuid: string | null;
};

/** The session's active account and its members (/v1/account). */
export const accountService = {
  async current() {
    return platformRequest<ActiveAccount>("GET", "/v1/account");
  },

  async rename(name: string) {
    return platformRequest<ActiveAccount>("PATCH", "/v1/account", { body: { name } });
  },

  async members() {
    return platformRequest<{ items: AccountMember[] }>("GET", "/v1/account/members");
  },

  async addMember(body: { email: string; role: AccountRole }) {
    return platformRequest<AccountMember>("POST", "/v1/account/members", { body });
  },

  async changeRole(userUuid: string, role: AccountRole) {
    return platformRequest<AccountMember>("PATCH", `/v1/account/members/${userUuid}`, {
      body: { role },
    });
  },

  async removeMember(userUuid: string) {
    return platformRequest<{ status: string }>("DELETE", `/v1/account/members/${userUuid}`);
  },

  /** Open (pending and expired) email invites of the active company account (TEC-945). */
  async invites() {
    return platformRequest<{ items: AccountInvite[]; total: number }>("GET", "/v1/account/invites", {
      query: { limit: 100 },
    });
  },

  async createInvite(body: { email: string; role: AccountRole }) {
    return platformRequest<AccountInvite>("POST", "/v1/account/invites", { body });
  },

  async resendInvite(uuid: string) {
    return platformRequest<AccountInvite>("POST", `/v1/account/invites/${uuid}/resend`);
  },

  async revokeInvite(uuid: string) {
    return platformRequest<{ status: string }>("DELETE", `/v1/account/invites/${uuid}`);
  },

  /** Public: what the invite link holder sees. */
  async previewInvite(token: string) {
    return platformRequest<InvitePreview>("GET", `/v1/invites/${encodeURIComponent(token)}`, {
      silent: true,
    });
  },

  async acceptInvite(token: string) {
    return platformRequest<InviteAcceptResult>(
      "POST",
      `/v1/invites/${encodeURIComponent(token)}/accept`,
      { silent: true },
    );
  },
};
