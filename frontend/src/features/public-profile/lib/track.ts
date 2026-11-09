"use client";

const SESSION_KEY = "cc_visit_session";

function sessionId(): string | undefined {
  try {
    let id = sessionStorage.getItem(SESSION_KEY);
    if (!id) {
      id = crypto.randomUUID();
      sessionStorage.setItem(SESSION_KEY, id);
    }
    return id;
  } catch {
    return undefined;
  }
}

/**
 * Fire-and-forget visitor event for a public profile (POST /v1/events via the BFF).
 * keepalive lets it survive navigation away (tel:, external links).
 */
export function trackProfileEvent(
  type: string,
  profileUuid: string,
  properties?: Record<string, string>,
) {
  try {
    void fetch("/api/v1/events", {
      method: "POST",
      keepalive: true,
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: JSON.stringify({
        type,
        subject: { type: "profile", id: profileUuid },
        session_id: sessionId(),
        properties: { ...properties, page: "public_profile" },
      }),
    }).catch(() => undefined);
  } catch {
    // analytics must never break the page
  }
}
