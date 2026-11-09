"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { BadgeCheck, Clock, ShieldAlert, ShieldCheck } from "lucide-react";
import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { permissions } from "@/config/permissions";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  type ProfileVerification,
  type ProfileVerificationStatus,
  profilesService,
} from "@/features/profiles/services/profiles.service";
import { isApiError } from "@/lib/api";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { appToast } from "@/providers/toast-provider";

const STATUS_STYLE: Record<ProfileVerificationStatus, string> = {
  none: "border-border text-muted-foreground",
  pending:
    "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
  verified:
    "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  rejected:
    "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300",
};

/** Owner side of the verified badge (TEC-549): status, request with a note, withdraw. */
export function ProfileVerificationCard({
  profileUuid,
}: {
  profileUuid: string;
}) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const canEdit =
    usePermission().can(permissions.profiles.write) && canManageAccount(user);
  const queryClient = useQueryClient();
  const [note, setNote] = useState("");

  const query = useQuery({
    queryKey: profilesKeys.verification(profileUuid),
    queryFn: () => profilesService.getVerification(profileUuid),
  });

  const onSaved = (data: ProfileVerification) => {
    queryClient.setQueryData(profilesKeys.verification(profileUuid), data);
    void queryClient.invalidateQueries({
      queryKey: profilesKeys.detail(profileUuid),
    });
  };
  const onError = (err: unknown) =>
    appToast.error(
      isApiError(err) ? err.message : t("profiles.verification.error"),
    );

  const request = useMutation({
    mutationFn: () =>
      profilesService.requestVerification(profileUuid, note.trim()),
    onSuccess: (data) => {
      onSaved(data);
      setNote("");
      appToast.success(t("profiles.verification.requested"));
    },
    onError,
  });
  const cancel = useMutation({
    mutationFn: () => profilesService.cancelVerification(profileUuid),
    onSuccess: (data) => {
      onSaved(data);
      appToast.success(t("profiles.verification.cancelled"));
    },
    onError,
  });

  const view = query.data;
  if (!view) return null;
  const req = view.request;
  const fmt = (iso?: string | null) =>
    iso
      ? new Date(iso).toLocaleString(locale === "tr" ? "tr-TR" : "en-GB", {
          dateStyle: "medium",
        })
      : "";
  const revokedForIdentity =
    req?.origin === "re_review" && view.status === "pending";

  return (
    <Card className="border-border shadow-sm">
      <CardHeader className="pb-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <CardTitle className="flex items-center gap-2 text-base">
            <ShieldCheck className="text-primary size-4" />
            {t("profiles.verification.title")}
          </CardTitle>
          <Badge variant="outline" className={STATUS_STYLE[view.status]}>
            {view.status === "verified" ? (
              <BadgeCheck className="size-3.5" />
            ) : null}
            {view.status === "pending" ? <Clock className="size-3.5" /> : null}
            {t(`profiles.verification.status.${view.status}`)}
          </Badge>
        </div>
        <CardDescription>
          {t(`profiles.verification.hint.${view.status}`)}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-3 pt-0 text-sm">
        {revokedForIdentity ? (
          <p className="flex items-start gap-2 rounded-md border border-amber-500/30 bg-amber-500/5 p-2.5 text-amber-800 dark:text-amber-200">
            <ShieldAlert className="mt-0.5 size-4 shrink-0" />
            {t("profiles.verification.re_review_notice")}
          </p>
        ) : null}
        {view.status === "rejected" && req?.review_reason ? (
          <p className="text-muted-foreground">
            <span className="text-foreground font-medium">
              {t("profiles.verification.reason")}:{" "}
            </span>
            {req.review_reason}
          </p>
        ) : null}
        {view.status === "none" &&
        req?.status === "revoked" &&
        req.revoke_reason ? (
          <p className="text-muted-foreground">
            <span className="text-foreground font-medium">
              {t("profiles.verification.revoked_reason")}:{" "}
            </span>
            {req.revoke_reason}
          </p>
        ) : null}
        {req && view.status !== "none" ? (
          <p className="text-muted-foreground text-xs">
            {t("profiles.verification.requested_at", {
              date: fmt(req.requested_at),
            })}
            {req.reviewed_at
              ? ` · ${t("profiles.verification.reviewed_at", { date: fmt(req.reviewed_at) })}`
              : ""}
          </p>
        ) : null}

        {canEdit && view.can_request ? (
          <div className="space-y-2">
            <Label htmlFor="verification-note">
              {t("profiles.verification.note_label")}
            </Label>
            <Textarea
              id="verification-note"
              value={note}
              maxLength={1000}
              rows={2}
              placeholder={t("profiles.verification.note_placeholder")}
              onChange={(e) => setNote(e.target.value)}
            />
            <Button
              size="sm"
              onClick={() => request.mutate()}
              disabled={request.isPending}
            >
              <ShieldCheck className="mr-1.5 size-4" />
              {t("profiles.verification.request")}
            </Button>
          </div>
        ) : null}
        {canEdit && view.status === "pending" && req?.origin === "owner" ? (
          <Button
            size="sm"
            variant="outline"
            onClick={() => cancel.mutate()}
            disabled={cancel.isPending}
          >
            {t("profiles.verification.cancel")}
          </Button>
        ) : null}
      </CardContent>
    </Card>
  );
}
