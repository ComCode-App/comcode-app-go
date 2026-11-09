import type { ReactNode } from "react";

import { FeedbackWidget } from "@/components/common/feedback-widget";
import { FullErrorTracking } from "@/lib/error-tracking/full-error-tracking";
import { AppProviders } from "@/providers/app-providers";

/**
 * Everything except visitor pages (`/{username}`, `/c/*`, `/s/*`): app, platform, guest, share.
 * Panel providers live here instead of the root layout so visitor pages stay light (TEC-858).
 */
export default function ShellLayout({ children }: { children: ReactNode }) {
  return (
    <>
      {/* Full error tracking SDK on panel pages; visitor pages keep the lazy catcher (TEC-952). */}
      <FullErrorTracking />
      <AppProviders>{children}</AppProviders>
      {/* Linear feedback widget: own button + iframe (Shadow DOM). */}
      <FeedbackWidget />
    </>
  );
}
