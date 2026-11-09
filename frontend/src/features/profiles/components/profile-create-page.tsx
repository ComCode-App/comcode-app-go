"use client";

import { useRouter } from "next/navigation";

import { EntityPage } from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { ProfileForm } from "@/features/profiles/components/profile-form";
import { useCreateProfile } from "@/features/profiles/hooks/use-profile-mutations";
import { splitProfileMedia } from "@/features/profiles/lib/local-media";
import type { ProfileFormValues } from "@/features/profiles/schemas/profile-form";
import { useLocale } from "@/providers/locale-provider";

export function ProfileCreatePage() {
  const { t } = useLocale();
  const router = useRouter();
  const createProfile = useCreateProfile();

  const handleSubmit = async (values: ProfileFormValues) => {
    const media = splitProfileMedia(values);
    const created = await createProfile.mutateAsync({
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
    });
    await media.uploadPending(created.uuid);
    router.push(routes.app.profiles.detail(created.uuid));
  };

  return (
    <EntityPage
      title={t("profiles.create_title")}
      description={t("profiles.create_subtitle")}
      permission={permissions.profiles.write}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("profiles.title"), href: routes.app.profiles.root },
        { label: t("profiles.create_title") },
      ]}
    >
      <div className="w-full py-2">
        <ProfileForm
          onSubmit={handleSubmit}
          isLoading={createProfile.isPending}
        />
      </div>
    </EntityPage>
  );
}
