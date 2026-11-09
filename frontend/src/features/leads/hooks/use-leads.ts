import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";

import {
  leadsService,
  type LeadListParams,
} from "@/features/leads/services/leads.service";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

export const leadsKeys = {
  all: ["app", "leads"] as const,
  list: (params: LeadListParams) => [...leadsKeys.all, "list", params] as const,
};

export function useLeads(params: LeadListParams) {
  return useQuery({
    queryKey: leadsKeys.list(params),
    queryFn: () => leadsService.list(params),
    placeholderData: keepPreviousData,
  });
}

export function useLeadMutations() {
  const qc = useQueryClient();
  const { t } = useLocale();
  const invalidate = () => qc.invalidateQueries({ queryKey: leadsKeys.all });

  const markRead = useMutation({
    mutationFn: (uuid: string) => leadsService.markRead(uuid),
    onSuccess: () => void invalidate(),
    meta: { silentErrors: true },
  });

  const remove = useMutation({
    mutationFn: (uuid: string) => leadsService.remove(uuid),
    onSuccess: () => {
      void invalidate();
      appToast.success(t("leads.toast.deleted"));
    },
    meta: { silentErrors: true },
    onError: () => appToast.error(t("leads.toast.delete_failed")),
  });

  return { markRead, remove };
}
