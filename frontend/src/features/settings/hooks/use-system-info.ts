import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { systemKeys } from "@/features/settings/hooks/query-keys";
import { systemService } from "@/features/settings/services/system.service";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";

export function useSystemInfo(options?: { refetchInterval?: number }) {
  return useQuery({
    queryKey: systemKeys.info(),
    queryFn: () => systemService.getInfo(),
    refetchInterval: options?.refetchInterval ?? 30000,
    staleTime: 10000,
  });
}

export function useSystemPing() {
  const queryClient = useQueryClient();
  const { t } = useLocale();

  return useMutation({
    mutationFn: () => systemService.ping(),
    onSuccess: (data) => {
      queryClient.setQueryData(systemKeys.info(), data);
      appToast.success(t("settings.system.toast_ping_success"));
    },
    onError: () => {
      appToast.error(t("settings.system.toast_ping_failed"));
    },
  });
}
