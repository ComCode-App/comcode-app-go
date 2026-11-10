import type { Metadata } from "next";

import { QRLoginFallback } from "@/features/auth/components/qr-login-fallback";

// The session id in the path is a short-lived secret: never leak it via Referer
// and never index the page.
export const metadata: Metadata = {
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

export default function QRLoginFallbackPage() {
  return <QRLoginFallback />;
}
