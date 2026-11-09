import { ApiError } from "@/lib/api/errors";

type Translate = (
  key: string,
  params?: Record<string, string | number>,
) => string;

export type StructureErrorContext = "office_delete" | "group_delete" | "save";

/**
 * Friendly copy for company-structure API failures. The backend stays the authority: a 403 means
 * the role or office scope changed under us, a 409 that the row is still referenced or duplicated.
 */
export function structureErrorMessage(
  err: unknown,
  t: Translate,
  context: StructureErrorContext,
) {
  if (err instanceof ApiError) {
    if (err.isPlanLimit) return err.message;
    if (err.status === 409) {
      return context === "save"
        ? t("company_structure.errors.conflict")
        : t(`company_structure.errors.${context}_in_use`);
    }
    if (err.status === 403) return t("company_structure.errors.forbidden");
    if (err.status === 404) return t("company_structure.errors.not_found");
    if (err.isValidation && err.message) return err.message;
  }
  return err instanceof Error && err.message
    ? err.message
    : t("common.error_generic");
}
