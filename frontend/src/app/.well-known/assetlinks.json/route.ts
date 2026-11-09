import {
  associationResponse,
  buildAssetLinks,
  readMobileAppConfig,
} from "@/lib/mobile/associated-domains";

// Runtime env (the image is prebuilt): never prerender this at build time.
export const dynamic = "force-dynamic";

/** Android App Links + login credential sharing (TEC-1183). 404 until MOBILE_ANDROID_SHA256_CERTS is set. */
export function GET() {
  const { status, body, headers } = associationResponse(
    buildAssetLinks(readMobileAppConfig(process.env)),
  );
  return new Response(body, { status, headers });
}
