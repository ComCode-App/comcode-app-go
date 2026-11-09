"use client";

import type { ReactNode } from "react";

import { RouteGuard } from "@/components/common/route-guard";
import { AppLayout } from "@/components/layout";

/** App shell — `/` */
export default function AppShellLayout({ children }: { children: ReactNode }) {
  return (
    <RouteGuard mode="app">
      <AppLayout variant="app">{children}</AppLayout>
    </RouteGuard>
  );
}
