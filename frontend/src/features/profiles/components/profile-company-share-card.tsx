"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Building2 } from "lucide-react";

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { permissions } from "@/config/permissions";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  profileSharesService,
  type ProfileCompanyShareTarget,
} from "@/features/profiles/services/profiles.service";
import { isApiError } from "@/lib/api";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

/**
 * "Share with my company" (TEC-1315): a personal profile can be shown read-only to the owners
 * and admins of each company the user belongs to. Off by default, one switch per company.
 * Hidden outside the personal account and when the user belongs to no company.
 */
export function ProfileCompanyShareCard({
  profileUuid,
}: {
  profileUuid: string;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const isPersonal = user?.account?.type === "personal";
  const canEdit = usePermission().can(permissions.profiles.write);
  const queryClient = useQueryClient();
  const key = profilesKeys.companyShares(profileUuid);

  const query = useQuery({
    queryKey: key,
    queryFn: () => profileSharesService.targets(profileUuid),
    enabled: isPersonal,
  });

  const toggle = useMutation({
    mutationFn: (v: { company: string; shared: boolean }) =>
      profileSharesService.setShared(profileUuid, v.company, v.shared),
    onSuccess: (_data, v) => {
      queryClient.setQueryData<{ items: ProfileCompanyShareTarget[] }>(
        key,
        (old) =>
          old && {
            items: old.items.map((it) =>
              it.company_uuid === v.company ? { ...it, shared: v.shared } : it,
            ),
          },
      );
      appToast.success(
        t(
          v.shared
            ? "profiles.company_share.shared_toast"
            : "profiles.company_share.unshared_toast",
        ),
      );
    },
    onError: (err) =>
      appToast.error(
        isApiError(err) ? err.message : t("profiles.company_share.error"),
      ),
  });

  const items = query.data?.items ?? [];
  if (!isPersonal || items.length === 0) return null;

  return (
    <Card className="border-border shadow-sm">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Building2 className="text-primary size-4" />
          {t("profiles.company_share.title")}
        </CardTitle>
        <CardDescription>
          {t("profiles.company_share.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 pt-0">
        {items.map((it) => {
          const id = `company-share-${it.company_uuid}`;
          return (
            <div
              key={it.company_uuid}
              className="border-border/60 flex items-start justify-between gap-4 rounded-lg border p-3"
            >
              <div className="min-w-0 space-y-1">
                <Label htmlFor={id} className="font-medium">
                  {t("profiles.company_share.switch_label", {
                    company: it.company_name,
                  })}
                </Label>
                <p className="text-muted-foreground text-xs">
                  {t("profiles.company_share.consent_hint")}
                </p>
              </div>
              <Switch
                id={id}
                checked={it.shared}
                disabled={!canEdit || toggle.isPending}
                onCheckedChange={(shared) =>
                  toggle.mutate({ company: it.company_uuid, shared })
                }
              />
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
