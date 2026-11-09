import type { Metadata } from "next";

import { brand } from "@/config/brand";
import { fontMono, fontSans } from "@/config/fonts";
import { ErrorCatcher } from "@/lib/error-tracking/error-catcher";
import { PublicProviders } from "@/providers/public-providers";

import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: brand.productName,
    template: `%s · ${brand.name}`,
  },
  description: brand.tagline,
  applicationName: brand.productName,
};

/**
 * Shared by visitor pages (`/{username}`, `/c/*`, `/s/*`) and the panel. Only lean providers here;
 * session, query cache, realtime, step-up and dialogs mount in `(shell)/layout.tsx` (TEC-858).
 */
export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="tr" suppressHydrationWarning className="h-full">
      <body
        className={`${fontSans.variable} ${fontMono.variable} min-h-full font-sans antialiased`}
      >
        {/* Lazy error catcher: loads the SDK only after the first error (TEC-952). */}
        <ErrorCatcher />
        <PublicProviders>{children}</PublicProviders>
      </body>
    </html>
  );
}
