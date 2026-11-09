export const companyStructureKeys = {
  all: ["company-structure"] as const,
  offices: () => [...companyStructureKeys.all, "offices"] as const,
  groups: (officeId?: string) =>
    officeId
      ? ([...companyStructureKeys.all, "groups", officeId] as const)
      : ([...companyStructureKeys.all, "groups"] as const),
  assignments: () => [...companyStructureKeys.all, "assignment"] as const,
  assignment: (userUuid: string) =>
    [...companyStructureKeys.assignments(), userUuid] as const,
  scopesAll: () => [...companyStructureKeys.all, "scopes"] as const,
  scopes: (userUuid: string) =>
    [...companyStructureKeys.scopesAll(), userUuid] as const,
  sector: () => [...companyStructureKeys.all, "sector"] as const,
  sectorCatalog: () => [...companyStructureKeys.all, "sector-catalog"] as const,
};
