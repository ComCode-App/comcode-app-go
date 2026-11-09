"use client";

import type { ReactNode } from "react";
import { PlatformSettingsLayout } from "@/features/settings";

export default function Layout({ children }: { children: ReactNode }) {
  return <PlatformSettingsLayout>{children}</PlatformSettingsLayout>;
}
