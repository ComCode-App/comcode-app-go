import { Suspense } from "react";

import { OrdersPage } from "@/features/orders";

export default function Page() {
  // OrdersPage reads ?open= (useSearchParams), which needs a Suspense boundary.
  return (
    <Suspense>
      <OrdersPage />
    </Suspense>
  );
}
