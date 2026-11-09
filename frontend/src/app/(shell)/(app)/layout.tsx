"use client";

import type { ReactNode } from "react";

import { PaymentFrameExit } from "@/components/common/payment-frame-exit";
import { RouteGuard } from "@/components/common/route-guard";
import { AppLayout } from "@/components/layout";

/** App shell — `/` */
export default function AppShellLayout({ children }: { children: ReactNode }) {
  return (
    <>
      {/* Before the guard: PayTR result pages load signed out inside its iFrame (TEC-1125). */}
      <PaymentFrameExit />
      <RouteGuard mode="app">
        <AppLayout variant="app">{children}</AppLayout>
      </RouteGuard>
    </>
  );
}
