import { Suspense } from "react";

import { RawEventsPage } from "@/features/platform-analytics";

export default function PlatformAnalyticsEventsPage() {
  return (
    <Suspense>
      <RawEventsPage />
    </Suspense>
  );
}
