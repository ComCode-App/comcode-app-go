"use client";

import { useQuery } from "@tanstack/react-query";

import { ErrorState } from "@/components/common/error-state";
import { Loading } from "@/components/common/loading";
import { PermissionGuard } from "@/components/common/permission-guard";
import {
  EntityDetail,
  EntityHeader,
  EntityPage,
  EntitySectionCard,
} from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { CompanyAddMemberForm } from "@/features/companies/components/company-add-member-form";
import { companiesKeys } from "@/features/companies/hooks/query-keys";
import {
  useAddCompanyMember,
  useChangeCompanyMemberRole,
  useRemoveCompanyMember,
  useUpdateCompany,
} from "@/features/companies/hooks/use-company-mutations";
import { companiesService } from "@/features/companies/services/companies.service";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";

export function CompanyDetailPage({ uuid }: { uuid: string }) {
  const { t } = useLocale();
  const { confirmDelete } = useDialogs();
  const updateCompany = useUpdateCompany(uuid);
  const changeRole = useChangeCompanyMemberRole(uuid);
  const removeMember = useRemoveCompanyMember(uuid);
  const addMember = useAddCompanyMember(uuid);
  const roles = ["owner", "admin", "member"] as const;

  const detailQuery = useQuery({
    queryKey: companiesKeys.detail(uuid),
    queryFn: () => companiesService.get(uuid),
  });

  const data = detailQuery.data;
  const title = data?.name ?? t("companies.detail_title");

  return (
    <EntityPage
      title={title}
      description={t("companies.detail_description")}
      permission={permissions.companies.read}
      forbiddenFallback={
        <ErrorState
          title={t("common.error_forbidden")}
          description={t("companies.forbidden")}
        />
      }
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.platform.home },
        {
          label: t("companies.title"),
          href: routes.platform.companies.root,
        },
        { label: title },
      ]}
      actions={
        data ? (
          <PermissionGuard permission={permissions.companies.write}>
            <Button
              size="sm"
              variant={data.status === "active" ? "destructive" : "outline"}
              disabled={updateCompany.isPending}
              onClick={() =>
                updateCompany.mutate({
                  status: data.status === "active" ? "suspended" : "active",
                })
              }
            >
              {data.status === "active"
                ? t("companies.actions.suspend")
                : t("companies.actions.activate")}
            </Button>
          </PermissionGuard>
        ) : null
      }
    >
      {detailQuery.isLoading ? <Loading label={t("common.loading")} /> : null}
      {detailQuery.isError ? (
        <ErrorState
          title={t("common.error_generic")}
          description={t("companies.detail_not_found")}
          onRetry={() => void detailQuery.refetch()}
          retryLabel={t("common.retry")}
        />
      ) : null}

      {data ? (
        <div className="space-y-6">
          <EntityHeader
            title={data.name}
            description={data.slug}
            badges={
              <Badge
                variant={data.status === "active" ? "outline" : "secondary"}
              >
                {t(`companies.status.${data.status}`)}
              </Badge>
            }
          />

          <EntitySectionCard title={t("companies.detail.profile")}>
            <EntityDetail
              sections={[
                {
                  id: "profile",
                  fields: [
                    {
                      key: "name",
                      label: t("companies.fields.name"),
                      value: data.name,
                    },
                    {
                      key: "slug",
                      label: t("companies.fields.slug"),
                      value: <code className="text-xs">{data.slug}</code>,
                    },
                    {
                      key: "status",
                      label: t("companies.fields.status"),
                      value: t(`companies.status.${data.status}`),
                    },
                    {
                      key: "uuid",
                      label: t("companies.fields.uuid"),
                      value: <code className="text-xs">{data.uuid}</code>,
                    },
                  ],
                },
              ]}
            />
          </EntitySectionCard>

          <EntitySectionCard title={t("companies.members")}>
            {data.members.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {t("companies.members_empty_description")}
              </p>
            ) : (
              <div className="rounded-md border">
                <table className="w-full text-sm">
                  <thead className="bg-muted/50 border-b">
                    <tr>
                      <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                        {t("companies.fields.user")}
                      </th>
                      <th className="text-muted-foreground h-10 px-4 text-left font-medium">
                        {t("companies.fields.role")}
                      </th>
                      <th className="h-10 px-4" />
                    </tr>
                  </thead>
                  <tbody>
                    {data.members.map((member) => (
                      <tr
                        key={member.user_uuid}
                        className="hover:bg-muted/50 border-b last:border-0"
                      >
                        <td className="p-4">
                          <div className="font-medium">
                            {member.name} {member.surname}
                          </div>
                          <div className="text-muted-foreground text-xs">
                            {member.email}
                          </div>
                        </td>
                        <td className="p-4">
                          <PermissionGuard
                            permission={permissions.companies.write}
                            fallback={t(`companies.roles.${member.role}`)}
                          >
                            <Select
                              value={member.role}
                              disabled={changeRole.isPending}
                              onValueChange={(role) =>
                                changeRole.mutate({
                                  userUuid: member.user_uuid,
                                  role,
                                })
                              }
                            >
                              <SelectTrigger
                                className="h-8 w-36"
                                aria-label={t("companies.fields.role")}
                              >
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {roles.map((role) => (
                                  <SelectItem key={role} value={role}>
                                    {t(`companies.roles.${role}`)}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </PermissionGuard>
                        </td>
                        <td className="p-4 text-right">
                          <PermissionGuard
                            permission={permissions.companies.write}
                          >
                            <Button
                              size="sm"
                              variant="ghost"
                              disabled={removeMember.isPending}
                              onClick={() =>
                                void confirmDelete({
                                  title: t("companies.remove_member_title"),
                                  confirmLabel: t(
                                    "companies.actions.remove_member",
                                  ),
                                  description: t(
                                    "companies.remove_member_description",
                                    {
                                      name: `${member.name} ${member.surname}`.trim(),
                                    },
                                  ),
                                }).then((confirmed) => {
                                  if (confirmed)
                                    removeMember.mutate(member.user_uuid);
                                })
                              }
                            >
                              {t("companies.actions.remove_member")}
                            </Button>
                          </PermissionGuard>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </EntitySectionCard>

          <PermissionGuard permission={permissions.companies.write}>
            <EntitySectionCard title={t("companies.add_member")}>
              <CompanyAddMemberForm
                isSubmitting={addMember.isPending}
                onSubmit={async (values) => {
                  await addMember.mutateAsync({
                    user_uuid: values.user_uuid,
                    role: values.role,
                  });
                }}
              />
            </EntitySectionCard>
          </PermissionGuard>
        </div>
      ) : null}
    </EntityPage>
  );
}
