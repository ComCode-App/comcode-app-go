import localFont from "next/font/local";

/*
 * Self-hosted so `next build` never fetches fonts.googleapis.com (parallel CI builds failed on it).
 * Variable woff2 files under `src/assets/fonts/`, built from the google/fonts repo (SIL OFL 1.1,
 * license texts next to the files), subset to the latin + latin-ext ranges (TR covered).
 */

/** Primary UI — geometric, premium SaaS feel; full Latin Extended (TR). */
export const fontSans = localFont({
  src: "../assets/fonts/PlusJakartaSans-Variable.woff2",
  variable: "--font-comcode-sans",
  weight: "400 700",
  style: "normal",
  display: "swap",
});

/** Code, IDs, tables — crisp mono. */
export const fontMono = localFont({
  src: "../assets/fonts/JetBrainsMono-Variable.woff2",
  variable: "--font-comcode-mono",
  weight: "100 800",
  style: "normal",
  display: "swap",
  // Metric-adjusting an Arial fallback would distort a monospace font; theme.css lists mono fallbacks.
  adjustFontFallback: false,
});
