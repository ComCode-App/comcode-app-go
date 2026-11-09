import { JetBrains_Mono, Plus_Jakarta_Sans } from "next/font/google";

/** Primary UI — geometric, premium SaaS feel; full Latin Extended (TR). */
export const fontSans = Plus_Jakarta_Sans({
  variable: "--font-comcode-sans",
  subsets: ["latin", "latin-ext"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});

/** Code, IDs, tables — crisp mono. */
export const fontMono = JetBrains_Mono({
  variable: "--font-comcode-mono",
  subsets: ["latin", "latin-ext"],
  display: "swap",
});
