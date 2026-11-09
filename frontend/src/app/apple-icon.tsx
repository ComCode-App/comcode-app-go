import { ImageResponse } from "next/og";

import { brand } from "@/config/brand";

export const size = { width: 180, height: 180 };
export const contentType = "image/png";

const COMCODE_ICON_PATH =
  "M1054.91,3906.1s-386.5,389.36-418.638,421.74-57.216,115.61,7.047,180.35,150.247,36.08,179.112,7,394.869-397.81,426.289-429.46,73.73-19.45,92.33-.71c15.7,15.82,39.86,54.98.68,94.45s-398.484,401.45-426.269,429.44c-42.434,42.75-208.849,146.16-360.2-6.33-152.3-153.43-54.268-327.31-12.621-369.27S843.6,3930.12,869.551,3903.97,980.925,3831.56,1054.91,3906.1Zm-76.821,734.86S1364.6,4251.58,1396.74,4219.2s57.22-115.61-7.05-180.35-150.24-36.08-179.11-7-394.881,397.82-426.3,429.47-73.728,19.45-92.327.72c-15.7-15.83-39.864-54.99-.683-94.46s398.494-401.46,426.284-429.45c42.43-42.75,208.85-146.16,360.21,6.32,152.3,153.43,54.26,327.32,12.61,369.28s-300.97,303.2-326.92,329.35S1052.07,4715.49,978.089,4640.96Z";

/** Apple touch / PWA icon — ComCode mark on brand gradient. */
export default function AppleIcon() {
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: brand.colors.gradient,
        borderRadius: 36,
      }}
    >
      <svg
        width="120"
        height="90"
        viewBox="0 0 1087 819"
        fill={brand.colors.white}
      >
        <path
          fillRule="evenodd"
          d={COMCODE_ICON_PATH}
          transform="translate(-473 -3864)"
        />
      </svg>
    </div>,
    { ...size },
  );
}
