"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { routes } from "@/config/routes";
import {
  type InvitePreview,
  accountService,
} from "@/features/accounts/services/account.service";
import { AuthCard } from "@/features/auth/components/auth-card";
import { isApiError } from "@/lib/api";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { authService } from "@/services/auth.service";

/** Maps the accept endpoint's error codes to messages. */
function acceptErrorKey(error: unknown): string {
  if (!isApiError(error)) return "accounts.invite_page.accept_failed";
  switch (error.code) {
    case "INVITE_EXPIRED":
      return "accounts.invite_page.expired";
    case "INVITE_CLOSED":
      return "accounts.invite_page.closed";
    case "INVITE_EMAIL_MISMATCH":
      return "accounts.invite_page.email_mismatch";
    case "INVITE_IMPERSONATION":
      return "accounts.invite_page.impersonating";
    case "RATE_LIMITED":
      return "accounts.invite_page.rate_limited";
    default:
      return "accounts.invite_page.accept_failed";
  }
}

/**
 * Invite link landing page (TEC-945). Anyone holding the link sees the invite. A signed-in user
 * whose email is the invited address accepts it and is switched into the company account; others
 * are sent to login or sign-up and come back here.
 */
export function InviteAcceptPage({ token }: { token: string }) {
  const { t } = useLocale();
  const { user, isAuthenticated, bootstrapped, hydrateProfile, logout } = useAuth();
  const [error, setError] = useState<string | null>(null);

  const preview = useQuery({
    queryKey: ["invite", token],
    queryFn: () => accountService.previewInvite(token),
    retry: false,
  });

  const here = routes.guest.invite(token);
  const accept = useMutation({
    mutationFn: async () => {
      const result = await accountService.acceptInvite(token);
      await authService.switchAccount(result.account_uuid);
      await hydrateProfile();
      return result;
    },
    onSuccess: () => {
      // App data is cached per account; reload into the company.
      window.location.assign(routes.app.account.root);
    },
    onError: (err) => setError(t(acceptErrorKey(err))),
  });

  if (preview.isLoading || !bootstrapped) {
    return <AuthCard title={t("accounts.invite_page.title")} description={t("common.loading")}>{null}</AuthCard>;
  }
  if (preview.isError || !preview.data) {
    return (
      <AuthCard
        title={t("accounts.invite_page.invalid_title")}
        description={
          isApiError(preview.error) && preview.error.code === "RATE_LIMITED"
            ? t("accounts.invite_page.rate_limited")
            : t("accounts.invite_page.invalid")
        }
      >
        <Button asChild className="w-full">
          <Link href={routes.app.home}>{t("accounts.invite_page.go_home")}</Link>
        </Button>
      </AuthCard>
    );
  }

  const invite: InvitePreview = preview.data;
  const role = t(`accounts.roles.${invite.role}`);
  const description = t("accounts.invite_page.description", {
    inviter: invite.invited_by_name || invite.account_name,
    account: invite.account_name,
    role,
  });

  if (invite.status !== "pending") {
    const key =
      invite.status === "expired"
        ? "accounts.invite_page.expired"
        : invite.status === "accepted"
          ? "accounts.invite_page.accepted"
          : "accounts.invite_page.closed";
    return (
      <AuthCard title={invite.account_name} description={t(key)}>
        <Button asChild className="w-full">
          <Link href={routes.app.home}>{t("accounts.invite_page.go_home")}</Link>
        </Button>
      </AuthCard>
    );
  }

  const next = encodeURIComponent(here);
  const loginHref = `${routes.guest.login}?next=${next}`;
  const registerHref = `${routes.guest.register}?invite=${encodeURIComponent(token)}&email=${encodeURIComponent(invite.email)}&next=${next}`;

  if (!isAuthenticated || !user) {
    return (
      <AuthCard title={invite.account_name} description={description}>
        <div className="space-y-3">
          <p className="text-muted-foreground text-center text-sm">
            {invite.user_exists
              ? t("accounts.invite_page.login_hint", { email: invite.email })
              : t("accounts.invite_page.signup_hint", { email: invite.email })}
          </p>
          <Button asChild className="w-full">
            <Link href={invite.user_exists ? loginHref : registerHref}>
              {invite.user_exists ? t("accounts.invite_page.login") : t("accounts.invite_page.signup")}
            </Link>
          </Button>
          <Button asChild variant="outline" className="w-full">
            <Link href={invite.user_exists ? registerHref : loginHref}>
              {invite.user_exists ? t("accounts.invite_page.signup") : t("accounts.invite_page.login")}
            </Link>
          </Button>
        </div>
      </AuthCard>
    );
  }

  if (user.email.toLowerCase() !== invite.email.toLowerCase()) {
    return (
      <AuthCard
        title={invite.account_name}
        description={t("accounts.invite_page.signed_in_as_other", { email: user.email, invited: invite.email })}
      >
        <Button
          className="w-full"
          onClick={() => {
            void logout().then(() => window.location.assign(loginHref));
          }}
        >
          {t("accounts.invite_page.switch_user")}
        </Button>
      </AuthCard>
    );
  }

  return (
    <AuthCard title={invite.account_name} description={description}>
      <div className="space-y-3">
        {error ? <p className="text-destructive text-center text-sm">{error}</p> : null}
        <Button className="w-full" disabled={accept.isPending} onClick={() => accept.mutate()}>
          {accept.isPending ? t("accounts.invite_page.accepting") : t("accounts.invite_page.accept")}
        </Button>
      </div>
    </AuthCard>
  );
}
