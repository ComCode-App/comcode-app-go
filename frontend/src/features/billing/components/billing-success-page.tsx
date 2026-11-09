"use client";

import Link from "next/link";
import { CheckCircle2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";

export function BillingSuccessPage() {
  const { t } = useLocale();

  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-6 pt-8">
      <Card>
        <CardHeader className="text-center">
          <CheckCircle2 className="text-primary mx-auto size-12" />
          <CardTitle className="mt-4">{t("billing.success.title")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4 text-center text-sm">
          <p className="text-muted-foreground">{t("billing.success.description")}</p>
          <Button asChild>
            <Link href={routes.app.billing.root}>{t("billing.success.back")}</Link>
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
