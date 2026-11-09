import { ApiError } from "@/lib/api/errors";

type Translate = (
  key: string,
  params?: Record<string, string | number>,
) => string;

/** Item errors of a job are stable codes (`company_bulk.item_errors.*`) or a validation message. */
export function itemErrorText(t: Translate, code: string) {
  const key = `company_bulk.item_errors.${code}`;
  const text = t(key);
  return text === key ? code : text;
}

export function requestErrorText(t: Translate, err: unknown) {
  if (err instanceof ApiError) {
    if (err.status === 403) return t("company_bulk.errors.forbidden");
    if (err.status === 404) return t("company_bulk.errors.not_found");
    if (err.status === 409) return t("company_bulk.errors.conflict");
    if (err.message) return err.message;
  }
  return err instanceof Error && err.message
    ? err.message
    : t("common.error_generic");
}
