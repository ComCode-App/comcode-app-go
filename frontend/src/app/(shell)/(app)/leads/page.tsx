import { Suspense } from "react";

import { LeadsPage } from "@/features/leads";

export default function LeadsRoute() {
  return (
    <Suspense>
      <LeadsPage />
    </Suspense>
  );
}
