"use client";

import { useQuery } from "@tanstack/react-query";
import { ExternalLink, Users } from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import { profileSharesService } from "@/features/profiles/services/profiles.service";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";

const PARAMS = { limit: 100, offset: 0 };

/**
 * "Member profiles" on the company /profiles page (TEC-1315): personal profiles members chose
 * to share with this company. Read-only, owners and admins only, hidden when empty.
 */
export function CompanySharedProfilesSection() {
  const { t } = useLocale();
  const { user } = useAuth();
  const visible =
    user?.account?.type === "company" && canManageAccount(user ?? null);

  const query = useQuery({
    queryKey: profilesKeys.companyShared(PARAMS),
    queryFn: () => profileSharesService.companyProfiles(PARAMS),
    enabled: visible,
  });

  const items = query.data?.items ?? [];
  if (!visible || items.length === 0) return null;

  return (
    <Card
      className="border-border shadow-sm"
      data-testid="company-shared-profiles"
    >
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-base">
            <Users className="text-primary size-4" />
            {t("profiles.shared_profiles.title")}
          </CardTitle>
          <Badge variant="outline">
            {t("profiles.shared_profiles.read_only")}
          </Badge>
        </div>
        <CardDescription>
          {t("profiles.shared_profiles.description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="grid grid-cols-1 gap-3 pt-0 sm:grid-cols-2 xl:grid-cols-3">
        {items.map((p) => {
          const fullName =
            `${p.first_name} ${p.last_name}`.trim() ||
            t("profiles.preview_card.untitled");
          const initials =
            `${p.first_name?.[0] || ""}${p.last_name?.[0] || ""}`.toUpperCase();
          const memberName =
            `${p.member.name} ${p.member.surname}`.trim() || p.member.email;
          return (
            <div
              key={p.uuid}
              className="border-border/60 bg-card flex items-start gap-3 rounded-xl border p-3"
            >
              <Avatar className="size-11 shrink-0">
                {p.avatar_url ? (
                  <AvatarImage src={p.avatar_url} alt={fullName} />
                ) : null}
                <AvatarFallback>{initials || "?"}</AvatarFallback>
              </Avatar>
              <div className="min-w-0 flex-1 space-y-0.5">
                <p className="truncate text-sm font-semibold">{fullName}</p>
                {p.title || p.company_name ? (
                  <p className="text-muted-foreground truncate text-xs">
                    {[p.title, p.company_name].filter(Boolean).join(" · ")}
                  </p>
                ) : null}
                <p className="text-muted-foreground truncate text-xs">
                  {t("profiles.shared_profiles.shared_by", {
                    name: memberName,
                  })}
                </p>
                {p.username ? (
                  <a
                    href={`/${p.username}`}
                    target="_blank"
                    rel="noreferrer"
                    className="text-primary inline-flex max-w-full items-center gap-1 text-xs hover:underline"
                  >
                    <ExternalLink className="size-3 shrink-0" />
                    <span className="truncate">/{p.username}</span>
                  </a>
                ) : null}
              </div>
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}
