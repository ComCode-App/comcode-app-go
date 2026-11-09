"use client";

import { ArrowRight, ListChecks } from "lucide-react";
import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { routes } from "@/config/routes";
import type { ProfileCompletionSuggestion } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

interface ProfileCompletionChecklistProps {
  profileUuid: string;
  rate: number;
  suggestions?: ProfileCompletionSuggestion[];
  /** Links open the editor tab; false renders plain items (read-only members). */
  canEdit?: boolean;
}

/**
 * "Complete your profile": the server's missing-field suggestions (highest gain first), each a
 * deep link to the editor tab that fixes it. Hidden once the profile is complete.
 */
export function ProfileCompletionChecklist({
  profileUuid,
  rate,
  suggestions,
  canEdit = true,
}: ProfileCompletionChecklistProps) {
  const { t } = useLocale();
  const items = suggestions ?? [];
  if (items.length === 0) return null;

  return (
    <Card className="border-border shadow-sm">
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <ListChecks className="text-primary size-4" />
          {t("profiles.completion.checklist_title")}
        </CardTitle>
        <CardDescription>
          {t("profiles.completion.checklist_desc")}
        </CardDescription>
        <div className="flex items-center gap-3 pt-1">
          <Progress
            value={rate}
            className="bg-muted/60 h-1.5 flex-1"
            aria-label={t("profiles.completion.rate_label")}
          />
          <span className="text-sm font-semibold tabular-nums">%{rate}</span>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        <ul className="divide-border divide-y">
          {items.map((s) => {
            const body = (
              <>
                <span className="flex-1 text-sm">{t(s.i18n_key)}</span>
                <Badge variant="secondary" className="font-mono text-xs">
                  +{s.points}
                </Badge>
                {canEdit ? (
                  <ArrowRight
                    aria-hidden
                    className="text-muted-foreground size-4"
                  />
                ) : null}
              </>
            );
            return (
              <li key={s.key}>
                {canEdit ? (
                  <Link
                    href={`${routes.app.profiles.edit(profileUuid)}?tab=${s.section}`}
                    className="hover:bg-muted/50 focus-visible:ring-ring -mx-2 flex items-center gap-3 rounded-md px-2 py-2.5 outline-none focus-visible:ring-2"
                  >
                    {body}
                  </Link>
                ) : (
                  <div className="flex items-center gap-3 py-2.5">{body}</div>
                )}
              </li>
            );
          })}
        </ul>
      </CardContent>
    </Card>
  );
}
