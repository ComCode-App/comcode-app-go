"use client";

import { useEffect, useState } from "react";

import { apiClient, fetchSession, unwrap } from "@/lib/api";
import { useAuthStore, type AuthUser } from "@/lib/auth/session-store";
import { mapMeToAuthUser, type Me } from "@/lib/auth/types";

/**
 * The signed-in visitor on a public page, if any. Visitor pages do not mount `AuthProvider`
 * (TEC-858), so this reuses the auth store when the visitor came from the panel and otherwise
 * checks the session lazily, only once `enabled` (e.g. the exchange form is opened).
 * Anonymous visitors resolve to `null` without any API call beyond the session check.
 */
export function useVisitorUser(enabled: boolean): { user: AuthUser | null; ready: boolean } {
  const storeUser = useAuthStore((s) => s.user);
  const storeBootstrapped = useAuthStore((s) => s.bootstrapped);
  const [fetched, setFetched] = useState<{ user: AuthUser | null } | null>(null);

  useEffect(() => {
    if (!enabled || storeBootstrapped || fetched) return;
    let cancelled = false;
    (async () => {
      let user: AuthUser | null = null;
      try {
        const session = await fetchSession();
        if (session.authenticated) {
          const me = await unwrap<Me>(await apiClient.GET("/v1/auth/me"), { silent: true });
          user = mapMeToAuthUser(me);
        }
      } catch {
        user = null; // the form still works without a session
      }
      if (!cancelled) setFetched({ user });
    })();
    return () => {
      cancelled = true;
    };
  }, [enabled, storeBootstrapped, fetched]);

  if (storeBootstrapped) return { user: storeUser, ready: true };
  return { user: fetched?.user ?? null, ready: fetched !== null };
}
