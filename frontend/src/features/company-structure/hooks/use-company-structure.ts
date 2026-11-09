"use client";

import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import {
  structureErrorMessage,
  type StructureErrorContext,
} from "../lib/errors";
import {
  type CompanyAssignmentInput,
  type CompanyGroupInput,
  type CompanyOfficeInput,
  type CompanySector,
  companyStructureService,
} from "../services/company-structure.service";
import { companyStructureKeys } from "./query-keys";

export function useCompanyOffices(enabled = true) {
  return useQuery({
    queryKey: companyStructureKeys.offices(),
    queryFn: companyStructureService.listOffices,
    enabled,
  });
}

/** `officeId`: an office UUID, or `"none"` for company-wide groups only. */
export function useCompanyGroups(enabled = true, officeId?: string) {
  return useQuery({
    queryKey: companyStructureKeys.groups(officeId),
    queryFn: () => companyStructureService.listGroups(officeId),
    enabled,
  });
}

/** Every member's office/group placement from one paginated list endpoint. */
export function useCompanyAssignments(enabled = true) {
  return useQuery({
    queryKey: companyStructureKeys.assignments(),
    queryFn: companyStructureService.listAssignments,
    enabled,
  });
}

export function useMemberScopes(userUuids: string[], enabled = true) {
  return useQueries({
    queries: userUuids.map((uuid) => ({
      queryKey: companyStructureKeys.scopes(uuid),
      queryFn: () => companyStructureService.getScopes(uuid),
      enabled,
    })),
  });
}

export function useCompanySector(enabled = true) {
  return useQuery({
    queryKey: companyStructureKeys.sector(),
    queryFn: companyStructureService.getSector,
    enabled,
  });
}

export function useSectorCatalog(enabled = true) {
  return useQuery({
    queryKey: companyStructureKeys.sectorCatalog(),
    queryFn: companyStructureService.listSectorCatalog,
    enabled,
    staleTime: 5 * 60_000,
  });
}

function useStructureMutationHelpers() {
  const qc = useQueryClient();
  const { t } = useLocale();
  const invalidate = () =>
    qc.invalidateQueries({ queryKey: companyStructureKeys.all });
  const onError = (context: StructureErrorContext) => (err: unknown) => {
    appToast.error(structureErrorMessage(err, t, context));
    // A 403 usually means the role or scopes changed; refresh what the UI derives hints from.
    void invalidate();
  };
  return { t, invalidate, onError };
}

export function useOfficeMutations() {
  const { t, invalidate, onError } = useStructureMutationHelpers();

  const create = useMutation({
    meta: { silentErrors: true },
    mutationFn: (body: CompanyOfficeInput) =>
      companyStructureService.createOffice(body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.offices.toast.created"));
    },
    onError: onError("save"),
  });
  const update = useMutation({
    meta: { silentErrors: true },
    mutationFn: ({ uuid, body }: { uuid: string; body: CompanyOfficeInput }) =>
      companyStructureService.updateOffice(uuid, body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.offices.toast.updated"));
    },
    onError: onError("save"),
  });
  const remove = useMutation({
    meta: { silentErrors: true },
    mutationFn: (uuid: string) => companyStructureService.deleteOffice(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.offices.toast.deleted"));
    },
    onError: onError("office_delete"),
  });
  return { create, update, remove };
}

export function useGroupMutations() {
  const { t, invalidate, onError } = useStructureMutationHelpers();

  const create = useMutation({
    meta: { silentErrors: true },
    mutationFn: (body: CompanyGroupInput) =>
      companyStructureService.createGroup(body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.groups.toast.created"));
    },
    onError: onError("save"),
  });
  const update = useMutation({
    meta: { silentErrors: true },
    mutationFn: ({ uuid, body }: { uuid: string; body: CompanyGroupInput }) =>
      companyStructureService.updateGroup(uuid, body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.groups.toast.updated"));
    },
    onError: onError("save"),
  });
  const remove = useMutation({
    meta: { silentErrors: true },
    mutationFn: (uuid: string) => companyStructureService.deleteGroup(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.groups.toast.deleted"));
    },
    onError: onError("group_delete"),
  });
  return { create, update, remove };
}

export function useAssignmentMutations() {
  const { t, invalidate, onError } = useStructureMutationHelpers();

  const setAssignment = useMutation({
    meta: { silentErrors: true },
    mutationFn: ({
      userUuid,
      body,
    }: {
      userUuid: string;
      body: CompanyAssignmentInput;
    }) => companyStructureService.setAssignment(userUuid, body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.members.toast.assignment_saved"));
    },
    onError: onError("save"),
  });
  const setScopes = useMutation({
    meta: { silentErrors: true },
    mutationFn: ({
      userUuid,
      officeUuids,
    }: {
      userUuid: string;
      officeUuids: string[];
    }) =>
      companyStructureService.setScopes(userUuid, {
        office_uuids: officeUuids,
      }),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.members.toast.scopes_saved"));
    },
    onError: onError("save"),
  });
  return { setAssignment, setScopes };
}

export function useSectorMutation() {
  const { t, invalidate, onError } = useStructureMutationHelpers();
  return useMutation({
    meta: { silentErrors: true },
    mutationFn: (body: CompanySector) =>
      companyStructureService.setSector(body),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("company_structure.sector.toast.saved"));
    },
    onError: onError("save"),
  });
}
