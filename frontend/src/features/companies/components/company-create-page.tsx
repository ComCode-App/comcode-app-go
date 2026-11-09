"use client";

import { useRouter } from "next/navigation";

import { EntityForm, EntityPage } from "@/components/entity";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { CompanyForm } from "@/features/companies/components/company-form";
import { useCreateCompany } from "@/features/companies/hooks/use-company-mutations";
import { useLocale } from "@/providers/locale-provider";

export function CompanyCreatePage() {
  const { t } = useLocale();
  const router = useRouter();
  const createCompany = useCreateCompany();

  return (
    <EntityPage
      title={t("companies.create_title")}
      description={t("companies.create_description")}
      permission={permissions.companies.write}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        {
          label: t("companies.title"),
          href: routes.platform.companies.root,
        },
        { label: t("companies.create_title") },
      ]}
    >
      <EntityForm>
        <CompanyForm
          isSubmitting={createCompany.isPending}
          onCancel={() => router.push(routes.platform.companies.root)}
          onSubmit={async (values) => {
            const company = await createCompany.mutateAsync({
              name: values.name,
              slug: values.slug || undefined,
              owner_user_uuid: values.owner_user_uuid,
            });
            router.push(routes.platform.companies.detail(company.uuid));
          }}
        />
      </EntityForm>
    </EntityPage>
  );
}
