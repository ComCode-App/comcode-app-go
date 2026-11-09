"use client";

import type { ReactNode } from "react";

// Panel i18n namespaces; visitor pages only ship the public ones (TEC-858).
import "@/lib/i18n/app-catalogs";

import { StepUpProvider } from "@/features/step-up-engine";
import { AuthProvider } from "@/providers/auth-provider";
import { LocaleHydrator } from "@/components/layout/locale-hydrator";
import { DialogProvider } from "@/providers/dialog-provider";
import { PermissionProvider } from "@/providers/permission-provider";
import { QueryProvider } from "@/providers/query-provider";
import { RealtimeProvider } from "@/providers/realtime-provider";

/**
 * Panel providers (app, platform, guest, share): session, query cache, permissions, step-up,
 * realtime and dialogs. Mounted by `app/(shell)/layout.tsx` inside the root `PublicProviders`,
 * so visitor pages (`/{username}`, `/c/*`, `/s/*`) never download them.
 */
export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <QueryProvider>
      <AuthProvider>
        <LocaleHydrator />
        <PermissionProvider>
          <StepUpProvider>
            <RealtimeProvider>
              <DialogProvider>{children}</DialogProvider>
            </RealtimeProvider>
          </StepUpProvider>
        </PermissionProvider>
      </AuthProvider>
    </QueryProvider>
  );
}
