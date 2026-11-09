import { Suspense } from "react";

import { ProfileEditPage } from "@/features/profiles";

export default function Page() {
  // ProfileEditPage reads ?tab= (useSearchParams), which needs a Suspense boundary.
  return (
    <Suspense>
      <ProfileEditPage />
    </Suspense>
  );
}
