"use client";

import { usePathname } from "next/navigation";
import Script from "next/script";

/**
 * First path segments of the app itself (frontend/src/app). Any other single-segment path is a
 * public profile (/{username}); /c/* are card links. Visitors there must not see the internal
 * Linear feedback widget.
 */
const APP_SEGMENTS = new Set([
  "", "account", "background-studio", "bank-accounts", "smart-links", "analytics", "billing", "cards", "orders", "profile",
  "profiles", "platform", "login", "forgot-password", "reset-password", "verify-email",
  "forbidden", "unauthorized", "share",
]);

function isVisitorPage(pathname: string): boolean {
  const first = pathname.split("/")[1] ?? "";
  return first === "c" || !APP_SEGMENTS.has(first);
}

/** Linear revision widget (issue.technowide.tr): app and platform pages only. */
export function FeedbackWidget() {
  const pathname = usePathname() ?? "/";
  if (isVisitorPage(pathname)) return null;
  return (
    <Script src="https://issue.technowide.tr/embed.js" strategy="afterInteractive" data-label="Geri bildirim" />
  );
}
