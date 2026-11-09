import type { NextRequest } from "next/server";

import { handlers } from "@/auth";
import { isBlockedCrossSiteAuthPost } from "@/lib/auth/apple-form-post";
import {
  allowedAppOrigins,
  bffErrorResponse,
  isCrossSiteMutation,
} from "@/lib/server/bff-guard";

export const { GET } = handlers;

/**
 * Cross-site POSTs to Auth.js are refused (same origin check as the BFF,
 * TEC-1102), except Apple's form_post callback, which is cross-site by design
 * and is protected by the state / PKCE / nonce check cookies (TEC-1123, see
 * `lib/auth/apple-form-post.ts`).
 */
export async function POST(request: NextRequest) {
  if (
    isBlockedCrossSiteAuthPost(
      request.method,
      request.nextUrl.pathname,
      isCrossSiteMutation(request, allowedAppOrigins(request.headers)),
    )
  ) {
    return bffErrorResponse(403, "FORBIDDEN", "cross-site request blocked");
  }
  return handlers.POST(request);
}
