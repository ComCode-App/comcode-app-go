import { redirect } from "next/navigation";

import { routes } from "@/config/routes";

/** `/company` has no page of its own; the first company section is offices. */
export default function CompanyRoute() {
  redirect(routes.app.company.offices);
}
