"use client";

import type { ReactNode } from "react";
import {
  SettingsMobileNav,
  SettingsSubSidebar,
} from "@/features/settings/components/settings-sub-sidebar";

export function PlatformSettingsLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-[calc(100svh-8.5rem)] flex-col gap-4 lg:flex-row lg:items-start lg:gap-6">
      {/* Mobile Top Navigation with Quick Pills Carousel & Drawer */}
      <SettingsMobileNav />

      {/* Desktop Sticky Sub-Sidebar */}
      <SettingsSubSidebar className="lg:sticky lg:top-20 lg:h-[calc(100svh-6rem)]" />

      {/* Page Content */}
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}
