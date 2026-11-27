"use client";

import { EyeOff } from "lucide-react";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import { useLocale } from "@/providers/locale-provider";

/** Every profile of the person is closed (TEC-1326): no contact data and no "save contact". */
export function ProfileNotShared() {
  const { t } = useLocale();
  return (
    <main className="bg-background flex min-h-dvh items-center justify-center px-4">
      <div className="max-w-sm text-center">
        <span className="bg-muted mx-auto flex size-14 items-center justify-center rounded-full">
          <EyeOff className="text-muted-foreground size-6" />
        </span>
        <h1 className="mt-4 text-xl font-semibold">
          {t("public_profile.not_shared_title")}
        </h1>
        <p className="text-muted-foreground mt-2 text-sm">
          {t("public_profile.not_shared_body")}
        </p>
        <Button asChild className="mt-6">
          <Link href="/" prefetch={false}>
            {t("public_profile.go_home")}
          </Link>
        </Button>
      </div>
    </main>
  );
}
