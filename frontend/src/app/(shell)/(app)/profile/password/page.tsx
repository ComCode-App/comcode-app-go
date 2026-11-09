import { redirect } from "next/navigation";

import { routes } from "@/config/routes";

export default function CmsProfilePasswordPage() {
  redirect(routes.app.profile.root);
}
