import { Suspense } from "react";

import { AccountPage } from "@/features/accounts";

export default function AccountRoute() {
  // AccountPage reads ?tab= with useSearchParams (TEC-1311).
  return (
    <Suspense>
      <AccountPage />
    </Suspense>
  );
}
