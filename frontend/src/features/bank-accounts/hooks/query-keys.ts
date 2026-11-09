export const bankAccountsQueryKeys = {
  all: ["app", "bank-accounts"] as const,
  lists: () => [...bankAccountsQueryKeys.all, "list"] as const,
  details: () => [...bankAccountsQueryKeys.all, "detail"] as const,
  detail: (uuid: string) => [...bankAccountsQueryKeys.details(), uuid] as const,
};
