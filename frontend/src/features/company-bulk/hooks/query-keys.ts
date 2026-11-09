export const companyBulkKeys = {
  all: ["company-bulk"] as const,
  meta: () => [...companyBulkKeys.all, "meta"] as const,
  members: (params: object) =>
    [...companyBulkKeys.all, "members", params] as const,
  jobs: () => [...companyBulkKeys.all, "jobs"] as const,
  job: (uuid: string) => [...companyBulkKeys.all, "job", uuid] as const,
};
