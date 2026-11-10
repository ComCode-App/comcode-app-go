import {
  associationResponse,
  buildAppleAppSiteAssociation,
  readMobileAppConfig,
} from "@/lib/mobile/associated-domains";

// Runtime env (the image is prebuilt): never prerender this at build time.
export const dynamic = "force-dynamic";

/** iOS universal links + shared web credentials (TEC-1183). 404 until MOBILE_APPLE_TEAM_ID is set. */
export function GET() {
  const { status, body, headers } = associationResponse(
    buildAppleAppSiteAssociation(readMobileAppConfig(process.env)),
  );
  return new Response(body, { status, headers });
}
