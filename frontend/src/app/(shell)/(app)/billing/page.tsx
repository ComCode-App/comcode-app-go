import { redirect } from "next/navigation";

import { routes } from "@/config/routes";

type Props = {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
};

/**
 * Subscription lives on the account page now (TEC-1311). Payment callbacks still land on
 * `/billing` (e.g. `?result=fail`), so the query is carried over.
 */
export default async function Page({ searchParams }: Props) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(await searchParams)) {
    if (key === "tab") continue;
    for (const v of Array.isArray(value) ? value : value ? [value] : [])
      query.append(key, v);
  }
  const rest = query.toString();
  redirect(
    rest ? `${routes.app.billing.root}&${rest}` : routes.app.billing.root,
  );
}
