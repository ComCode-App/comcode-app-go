"use client";

import { useEffect, type ReactNode } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { routes } from "@/config/routes";
import {
  defaultHomeForUser,
  isPlatformUser,
  isSubscriber,
} from "@/lib/auth/types";
import { loginWithNext, safeNextPath } from "@/lib/auth/safe-next";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";

type RouteGuardProps = {
  children: ReactNode;
  mode: "guest" | "platform" | "app";
};

function PageLoader({ label }: { label: string }) {
  return (
    <div className="text-muted-foreground flex min-h-svh items-center justify-center text-sm">
      {label}
    </div>
  );
}

function RoleMismatch({ expected, home }: { expected: string; home: string }) {
  const { t } = useLocale();
  return (
    <div className="flex min-h-svh items-center justify-center p-6">
      <Card className="w-full max-w-md">
        <CardHeader>
          <CardTitle>{t("errors.forbidden_title")}</CardTitle>
          <CardDescription>{t("errors.role_mismatch")}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <p className="text-muted-foreground text-sm">
            {t("errors.area_requires", { area: expected })}
          </p>
          <Button asChild>
            <Link href={home}>{t("errors.continue")}</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}

export function RouteGuard({ children, mode }: RouteGuardProps) {
  const router = useRouter();
  const { bootstrapped, isAuthenticated, user } = useAuth();
  const { t } = useLocale();

  useEffect(() => {
    if (!bootstrapped) return;

    if (mode === "guest" && isAuthenticated && user) {
      // Same target as the login form: a safe ?next= wins, otherwise the
      // user's default shell. Without this the guard races the form after
      // sign-in and drops the deep link.
      const next = new URLSearchParams(window.location.search).get("next");
      router.replace(safeNextPath(next, defaultHomeForUser(user)));
      return;
    }

    if ((mode === "platform" || mode === "app") && !isAuthenticated) {
      // Come back to the same page (query included) after signing in.
      const { pathname, search } = window.location;
      router.replace(loginWithNext(routes.guest.login, `${pathname}${search}`));
    }
  }, [bootstrapped, mode, isAuthenticated, user, router]);

  if (!bootstrapped) {
    return <PageLoader label={t("common.loading")} />;
  }

  if ((mode === "platform" || mode === "app") && !isAuthenticated) {
    // The effect above is sending the visitor to the login page.
    return <PageLoader label={t("errors.redirecting_to_login")} />;
  }

  if (mode === "platform" && user && !isPlatformUser(user)) {
    return (
      <RoleMismatch
        expected={t("layout.nav_platform")}
        home={isSubscriber(user) ? routes.app.home : routes.errors.forbidden}
      />
    );
  }

  if (mode === "app" && user && !isSubscriber(user)) {
    return (
      <RoleMismatch
        expected={t("errors.area_app")}
        home={
          isPlatformUser(user) ? routes.platform.home : routes.errors.forbidden
        }
      />
    );
  }

  return <>{children}</>;
}
