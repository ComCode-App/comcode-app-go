import localFont from "next/font/local";

/**
 * Heading serif of the `elegant_serif` theme (TEC-601). `preload: false` keeps it out of every
 * public page's critical path: the browser only downloads it when a heading actually uses it.
 * Self-hosted (unmodified google/fonts variable font, SIL OFL 1.1, see `src/assets/fonts/`).
 */
export const fontProfileSerif = localFont({
  src: "../../../assets/fonts/PlayfairDisplay-Variable.woff2",
  variable: "--font-pp-serif",
  weight: "400 900",
  style: "normal",
  display: "swap",
  preload: false,
  adjustFontFallback: "Times New Roman",
});
