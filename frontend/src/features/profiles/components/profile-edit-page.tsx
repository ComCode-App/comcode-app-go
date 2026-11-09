"use client";

import { useQuery } from "@tanstack/react-query";
import { Download, Eye, Star } from "lucide-react";
import { useParams, useRouter, useSearchParams } from "next/navigation";

import { ErrorState } from "@/components/common/error-state";
import { PageLoader } from "@/components/common/loading";
import { EntityPage } from "@/components/entity";
import { Button } from "@/components/ui/button";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { ProfileCompletionBadge } from "@/features/profiles/components/profile-completion-badge";
import { ProfileCompletionChecklist } from "@/features/profiles/components/profile-completion-checklist";
import { ProfileForm } from "@/features/profiles/components/profile-form";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  useSetPrimaryProfile,
  useUpdateProfile,
} from "@/features/profiles/hooks/use-profile-mutations";
import { splitProfileMedia } from "@/features/profiles/lib/local-media";
import type { ProfileFormValues } from "@/features/profiles/schemas/profile-form";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

export function ProfileEditPage() {
  const { t } = useLocale();
  const router = useRouter();
  const params = useParams<{ uuid: string }>();
  const uuid = params?.uuid ?? "";
  const tab = useSearchParams().get("tab");

  const updateProfile = useUpdateProfile();
  const setPrimary = useSetPrimaryProfile();

  const profileQuery = useQuery({
    queryKey: profilesKeys.detail(uuid),
    queryFn: () => profilesService.get(uuid),
    enabled: Boolean(uuid),
  });

  if (profileQuery.isLoading) {
    return <PageLoader label={t("common.loading")} />;
  }

  if (profileQuery.isError || !profileQuery.data) {
    return (
      <ErrorState
        title={t("common.error")}
        description={profileQuery.error?.message || t("profiles.empty_title")}
        retryLabel={t("common.back")}
        onRetry={() => router.push(routes.app.profiles.root)}
      />
    );
  }

  const profile = profileQuery.data;
  const primaryTrans =
    profile.translations?.find((t) => t.locale === profile.primary_locale) ||
    profile.translations?.[0];
  const displayName =
    `${primaryTrans?.first_name || ""} ${primaryTrans?.last_name || ""}`.trim() ||
    "Profile Studio";

  const handleSubmit = async (values: ProfileFormValues) => {
    const media = splitProfileMedia(values);
    await updateProfile.mutateAsync({
      uuid,
      body: {
        primary_locale: values.primary_locale,
        birthday: values.birthday || null,
        gender: values.gender || "",
        avatar_url: media.avatar_url,
        cover_url: media.cover_url,
        is_primary: values.is_primary,
        translations: values.translations,
        contact_details: values.contact_details,
        addresses: values.addresses,
        social_links: values.social_links,
      },
    });
    await media.uploadPending(uuid);
  };

  const handleDownloadVCard = async () => {
    const filename =
      `${primaryTrans?.first_name || "contact"}_${primaryTrans?.last_name || ""}`.trim();
    await profilesService.downloadVCard(
      profile.uuid,
      filename,
      profile.primary_locale,
    );
  };

  const handleSetPrimary = async () => {
    await setPrimary.mutateAsync(profile.uuid);
  };

  return (
    <EntityPage
      title={`Edit ${displayName}`}
      description={t("profiles.edit_subtitle")}
      permission={permissions.profiles.write}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("profiles.title"), href: routes.app.profiles.root },
        { label: displayName },
      ]}
      actions={
        <div className="flex items-center gap-2">
          <ProfileCompletionBadge
            rate={profile.completion_rate}
            className="mr-2"
          />
          <Button
            variant="outline"
            size="sm"
            onClick={() =>
              router.push(routes.app.profiles.detail(profile.uuid))
            }
          >
            <Eye className="mr-1.5 h-4 w-4" />
            View Card
          </Button>
          {!profile.is_primary && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleSetPrimary}
              disabled={setPrimary.isPending}
            >
              <Star className="mr-1.5 h-4 w-4" />
              {t("profiles.actions.set_primary")}
            </Button>
          )}
          <Button variant="outline" size="sm" onClick={handleDownloadVCard}>
            <Download className="mr-1.5 h-4 w-4" />
            {t("profiles.actions.download_vcard")}
          </Button>
        </div>
      }
    >
      <div className="w-full space-y-4 py-2">
        <ProfileCompletionChecklist
          profileUuid={profile.uuid}
          rate={profile.completion_rate}
          suggestions={profile.completion_suggestions}
        />
        <ProfileForm
          initialData={profile}
          onSubmit={handleSubmit}
          isLoading={updateProfile.isPending}
          initialTab={tab}
        />
      </div>
    </EntityPage>
  );
}
