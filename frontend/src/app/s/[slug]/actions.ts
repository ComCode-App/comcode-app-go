"use server";

import { cookies } from "next/headers";

import { SMART_LINK_PASSCODE_COOKIE, unlockUpstream } from "./resolve";

const PASSCODE_TTL_SECONDS = 60 * 60;
const SLUG_RE = /^[a-z0-9._-]{1,120}$/i;

export type UnlockResult = { ok: true } | { ok: false; status: number };

/**
 * Checks a passcode with the API (which rate limits wrong ones per visitor IP) and, when it is
 * right, keeps it in an httpOnly cookie for this link only, so /s/{slug} can render the
 * profile in place instead of redirecting to the public profile URL.
 */
export async function unlockSmartLink(
  slug: string,
  passcode: string,
): Promise<UnlockResult> {
  if (!SLUG_RE.test(slug) || !passcode || passcode.length > 64)
    return { ok: false, status: 400 };
  const result = await unlockUpstream(slug, passcode);
  if (result.status !== 200) return { ok: false, status: result.status };
  (await cookies()).set(SMART_LINK_PASSCODE_COOKIE, passcode, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: `/s/${slug}`,
    maxAge: PASSCODE_TTL_SECONDS,
  });
  return { ok: true };
}
