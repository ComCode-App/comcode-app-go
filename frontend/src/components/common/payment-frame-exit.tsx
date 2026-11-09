"use client";

import { useEffect } from "react";

import { isPaymentResultPath } from "@/config/security-headers";

/**
 * PayTR opens `merchant_ok_url` / `merchant_fail_url` inside its own iFrame
 * (TEC-1125). Move that result page to the top window, where the visitor's
 * session cookies are sent (inside the cross-site frame they are not, so the
 * route guard would otherwise send the frame to the login page).
 *
 * Rendered by the app shell layout OUTSIDE the route guard, so it runs before
 * sign-in is checked. Only on payment result paths (see `security-headers.ts`,
 * the only app pages PayTR may frame).
 */
export function PaymentFrameExit() {
  useEffect(() => {
    if (window.top === window.self || !window.top) return;
    if (!isPaymentResultPath(window.location.pathname)) return;
    try {
      window.top.location.href = window.location.href;
    } catch {
      // The top window is not ours (should not happen: frame-ancestors only
      // allows 'self' and PayTR above these pages). Leave the page as it is.
    }
  }, []);
  return null;
}
