import { useQuery } from "@tanstack/react-query";

import { companiesKeys } from "@/features/companies/hooks/query-keys";
import { companiesService } from "@/features/companies/services/companies.service";

export type CompaniesNavStats = {
  total: number;
};

export function useCompaniesNavStats(enabled = true) {
  return useQuery({
    queryKey: companiesKeys.navStats(),
    queryFn: async (): Promise<CompaniesNavStats> => {
      const result = await companiesService.list({ limit: 1, offset: 0 });
      return { total: result.total };
    },
    enabled,
    staleTime: 60 * 1000,
  });
}
