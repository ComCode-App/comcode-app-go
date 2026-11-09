import type { components } from "@/generated/api";
import { platformRequest } from "@/lib/api/platform-request";

export type CompanyOffice = components["schemas"]["CompanyOffice"];
export type CompanyOfficeInput = components["schemas"]["CompanyOfficeInput"];
export type CompanyGroup = components["schemas"]["CompanyGroup"];
export type CompanyGroupInput = components["schemas"]["CompanyGroupInput"];
export type CompanyGroupKind = CompanyGroup["kind"];
export type CompanyAssignment = components["schemas"]["CompanyAssignment"];
export type CompanyAssignmentInput =
  components["schemas"]["CompanyAssignmentInput"];
export type CompanyScopes = components["schemas"]["CompanyScopes"];
export type CompanySector = components["schemas"]["CompanySector"];
export type Sector = components["schemas"]["Sector"];

type Page<T> = { items: T[]; total: number; limit: number; offset: number };

/** API maximum page size for company structure lists. */
const PAGE_LIMIT = 100;
/** Hard stop so a misbehaving total can never loop forever. */
const MAX_PAGES = 50;

/** Reads every page of a limit/offset list (offices, groups and sectors are small catalogs). */
async function listAll<T>(
  path: string,
  query?: Record<string, string | undefined>,
) {
  const items: T[] = [];
  for (let page = 0; page < MAX_PAGES; page += 1) {
    const res = await platformRequest<Page<T>>("GET", path, {
      query: { ...query, limit: PAGE_LIMIT, offset: page * PAGE_LIMIT },
    });
    items.push(...(res.items ?? []));
    if (!res.items?.length || items.length >= res.total) break;
  }
  return items;
}

/** Active company account structure (/v1/company/*, docs/COMPANIES.md). */
export const companyStructureService = {
  listOffices() {
    return listAll<CompanyOffice>("/v1/company/offices");
  },
  createOffice(body: CompanyOfficeInput) {
    return platformRequest<CompanyOffice>("POST", "/v1/company/offices", {
      body,
      silent: true,
    });
  },
  updateOffice(uuid: string, body: CompanyOfficeInput) {
    return platformRequest<CompanyOffice>(
      "PUT",
      `/v1/company/offices/${uuid}`,
      { body, silent: true },
    );
  },
  deleteOffice(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/company/offices/${uuid}`, {
      silent: true,
    });
  },

  /**
   * Groups of the active company. `officeId` narrows to one office's groups;
   * `"none"` lists only company-wide groups.
   */
  listGroups(officeId?: string) {
    return listAll<CompanyGroup>("/v1/company/groups", {
      office_id: officeId,
    });
  },
  createGroup(body: CompanyGroupInput) {
    return platformRequest<CompanyGroup>("POST", "/v1/company/groups", {
      body,
      silent: true,
    });
  },
  updateGroup(uuid: string, body: CompanyGroupInput) {
    return platformRequest<CompanyGroup>("PUT", `/v1/company/groups/${uuid}`, {
      body,
      silent: true,
    });
  },
  deleteGroup(uuid: string) {
    return platformRequest<void>("DELETE", `/v1/company/groups/${uuid}`, {
      silent: true,
    });
  },

  /** Office/group placement of every member of the active company, in one list. */
  listAssignments() {
    return listAll<CompanyAssignment>("/v1/company/assignments");
  },
  getAssignment(userUuid: string) {
    return platformRequest<CompanyAssignment>(
      "GET",
      `/v1/company/members/${userUuid}/assignment`,
    );
  },
  setAssignment(userUuid: string, body: CompanyAssignmentInput) {
    return platformRequest<CompanyAssignment>(
      "PUT",
      `/v1/company/members/${userUuid}/assignment`,
      { body, silent: true },
    );
  },

  getScopes(userUuid: string) {
    return platformRequest<CompanyScopes>(
      "GET",
      `/v1/company/members/${userUuid}/scopes`,
    );
  },
  setScopes(userUuid: string, body: CompanyScopes) {
    return platformRequest<CompanyScopes>(
      "PUT",
      `/v1/company/members/${userUuid}/scopes`,
      {
        body,
        silent: true,
      },
    );
  },

  getSector() {
    return platformRequest<CompanySector>("GET", "/v1/company/sector");
  },
  setSector(body: CompanySector) {
    return platformRequest<CompanySector>("PUT", "/v1/company/sector", {
      body,
      silent: true,
    });
  },
  /** Read-only sector catalog for company accounts. */
  listSectorCatalog() {
    return listAll<Sector>("/v1/sectors");
  },
};
