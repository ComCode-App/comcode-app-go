import { Playfair_Display } from "next/font/google";

/**
 * Heading serif of the `elegant_serif` theme (TEC-601). `preload: false` keeps it out of every
 * public page's critical path: the browser only downloads it when a heading actually uses it.
 */
export const fontProfileSerif = Playfair_Display({
  variable: "--font-pp-serif",
  subsets: ["latin", "latin-ext"],
  weight: ["500", "600", "700"],
  display: "swap",
  preload: false,
});
