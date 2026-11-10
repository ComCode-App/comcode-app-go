"use client";

import { Centrifuge, type Subscription } from "centrifuge";
import { Loader2, RefreshCw, Smartphone } from "lucide-react";
import { signIn } from "next-auth/react";
import QRCode from "qrcode";
import { useCallback, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { realtimeConfig } from "@/config/realtime";
import { signInErrorKey } from "@/features/auth/lib/sign-in-errors";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { authService, type QRLoginCreated } from "@/services/auth.service";

/** Automatic refreshes before the user has to click (about 10 minutes of QR codes). */
const MAX_AUTO_REFRESHES = 5;
/** Ask for a fresh QR this long before the current one expires (120 s). */
const REFRESH_LEAD_MS = 5_000;
/** Never schedule a refresh sooner than this (clock skew, very short sessions). */
const MIN_REFRESH_DELAY_MS = 5_000;

type Phase =
  | "loading"
  | "ready"
  | "scanned"
  | "signing_in"
  | "rejected"
  | "expired"
  | "error"
  | "rate_limited"
  | "unavailable";

type QREvent = { type?: string; exchange_token?: string };

/**
 * QR sign-in on the web login page (TEC-1185, docs/design/mobile.md §3).
 *
 * The tab creates a short-lived session and gets an anonymous Centrifugo
 * connection token for its own `qrlogin:*` channel, then waits for the mobile
 * app to scan and approve it. No polling: the state endpoint is read once per
 * (re)subscribe so an event sent while the socket was down is not lost.
 *
 * This is the only component that opens its own Centrifugo client: the
 * app-wide ConnectionManager needs a signed-in user.
 *
 * On approval the tab signs in through the NextAuth `qr-login` provider, which
 * calls the exchange on the server, so the API tokens never reach this code.
 */
export function QRLoginPanel({
  onSignedIn,
}: {
  /** Finishes the login (profile hydrate + redirect); false when it failed. */
  onSignedIn: () => Promise<boolean>;
}) {
  const { t } = useLocale();
  const [phase, setPhase] = useState<Phase>("loading");
  const [errorKey, setErrorKey] = useState("auth.qr.error");
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);
  const sessionRef = useRef<QRLoginCreated | null>(null);
  const clientRef = useRef<Centrifuge | null>(null);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const refreshesRef = useRef(0);
  const exchangingRef = useRef(false);
  const mountedRef = useRef(true);
  const startRef = useRef<(manual: boolean) => void>(() => undefined);
  // The parent callback changes every render; keep it out of the effect deps so
  // a re-render never restarts the QR session.
  const onSignedInRef = useRef(onSignedIn);
  useEffect(() => {
    onSignedInRef.current = onSignedIn;
  }, [onSignedIn]);

  const teardown = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = null;
    clientRef.current?.disconnect();
    clientRef.current = null;
  }, []);

  const fail = useCallback((key: string) => {
    if (!mountedRef.current) return;
    setErrorKey(key);
    setPhase("error");
  }, []);

  const exchange = useCallback(
    async (session: QRLoginCreated, exchangeToken: string) => {
      if (exchangingRef.current || sessionRef.current !== session) return;
      exchangingRef.current = true;
      teardown();
      setPhase("signing_in");
      try {
        const result = await signIn("qr-login", {
          session_id: session.session_id,
          browser_secret: session.browser_secret,
          exchange_token: exchangeToken,
          redirect: false,
        });
        if (!result || result.error || result.code) {
          fail(signInErrorKey(result?.code ?? result?.error, "auth.qr.error"));
          return;
        }
        // The parent shows its own error when finishing fails (role mismatch).
        const ok = await onSignedInRef.current();
        if (!ok) fail("auth.qr.error");
      } catch {
        fail("auth.qr.error");
      } finally {
        exchangingRef.current = false;
      }
    },
    [fail, teardown],
  );

  const handleEvent = useCallback(
    (session: QRLoginCreated, event: QREvent) => {
      if (sessionRef.current !== session) return;
      switch (event.type) {
        case "scanned":
          setPhase((p) => (p === "ready" ? "scanned" : p));
          break;
        case "approved":
          if (event.exchange_token) {
            void exchange(session, event.exchange_token);
          }
          break;
        case "rejected":
          teardown();
          setPhase("rejected");
          break;
      }
    },
    [exchange, teardown],
  );

  const start = useCallback(
    async (manual: boolean) => {
      teardown();
      sessionRef.current = null;
      if (manual) refreshesRef.current = 0;
      setPhase("loading");

      let session: QRLoginCreated;
      let dataUrl: string;
      try {
        session = await authService.createQRLoginSession();
        dataUrl = await QRCode.toDataURL(session.qr_url, {
          width: 224,
          margin: 1,
          errorCorrectionLevel: "M",
        });
      } catch (err) {
        if (!mountedRef.current) return;
        if (isApiError(err) && err.code === "REALTIME_DISABLED") {
          setPhase("unavailable");
        } else if (isApiError(err) && err.code === "RATE_LIMITED") {
          setPhase("rate_limited");
        } else {
          fail("auth.qr.error");
        }
        return;
      }
      if (!mountedRef.current) return;
      sessionRef.current = session;
      setQrDataUrl(dataUrl);
      setPhase("ready");

      const client = new Centrifuge(
        session.realtime.ws_url || realtimeConfig.wsUrl,
        {
          token: session.realtime.connection_token,
          minReconnectDelay: realtimeConfig.reconnect.minDelayMs,
          maxReconnectDelay: realtimeConfig.reconnect.maxDelayMs,
        },
      );
      const sub: Subscription = client.newSubscription(
        session.realtime.channel,
        { token: session.realtime.subscription_token },
      );
      sub.on("publication", (ctx) => handleEvent(session, ctx.data as QREvent));
      // Catch up once per (re)subscribe in case an event was published while the
      // socket was down. Runs only on subscribe, so it is not a poll.
      sub.on("subscribed", () => {
        void authService
          .getQRLoginState(session.session_id, session.browser_secret)
          .then((state) => {
            if (state.status === "approved" && state.exchange_token) {
              handleEvent(session, {
                type: "approved",
                exchange_token: state.exchange_token,
              });
            } else if (
              state.status === "scanned" ||
              state.status === "rejected"
            ) {
              handleEvent(session, { type: state.status });
            }
          })
          .catch(() => undefined);
      });
      sub.subscribe();
      client.connect();
      clientRef.current = client;

      const msLeft =
        new Date(session.expires_at).getTime() - Date.now() - REFRESH_LEAD_MS;
      timerRef.current = setTimeout(
        () => {
          if (exchangingRef.current || sessionRef.current !== session) return;
          const hidden =
            typeof document !== "undefined" &&
            document.visibilityState === "hidden";
          if (hidden || refreshesRef.current >= MAX_AUTO_REFRESHES) {
            // A hidden or long-idle tab stops creating sessions until it is used.
            teardown();
            sessionRef.current = null;
            setPhase("expired");
            return;
          }
          refreshesRef.current += 1;
          startRef.current(false);
        },
        Math.max(msLeft, MIN_REFRESH_DELAY_MS),
      );
    },
    [fail, handleEvent, teardown],
  );

  useEffect(() => {
    startRef.current = (manual) => void start(manual);
  }, [start]);

  useEffect(() => {
    mountedRef.current = true;
    // Deferred so no state update runs synchronously inside the effect.
    const first = setTimeout(() => void start(true), 0);
    return () => {
      clearTimeout(first);
      mountedRef.current = false;
      sessionRef.current = null;
      teardown();
    };
  }, [start, teardown]);

  // A tab that went idle while hidden gets a fresh QR when it is shown again.
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === "visible" && phase === "expired") {
        startRef.current(true);
      }
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [phase]);

  const showQR = phase === "ready" || phase === "scanned";
  const message =
    phase === "rejected"
      ? t("auth.qr.rejected")
      : phase === "expired"
        ? t("auth.qr.expired")
        : phase === "rate_limited"
          ? t("auth.login.rate_limited")
          : phase === "unavailable"
            ? t("auth.qr.unavailable")
            : phase === "error"
              ? t(errorKey)
              : null;

  return (
    <div className="flex flex-col items-center gap-4" data-qr-phase={phase}>
      <div className="relative flex h-[240px] w-[240px] items-center justify-center rounded-xl border bg-white p-2">
        {showQR && qrDataUrl ? (
          // Data URL from `qrcode`; next/image adds nothing here.
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={qrDataUrl}
            alt={t("auth.qr.alt")}
            width={224}
            height={224}
            className={phase === "scanned" ? "opacity-10" : undefined}
          />
        ) : null}
        {phase === "loading" || phase === "signing_in" ? (
          <Loader2
            className="size-6 animate-spin text-neutral-500"
            aria-hidden
          />
        ) : null}
        {phase === "scanned" ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 p-4 text-center text-neutral-900">
            <Smartphone className="size-8" aria-hidden />
            <p className="text-sm font-medium" role="status">
              {t("auth.qr.scanned")}
            </p>
          </div>
        ) : null}
        {message ? (
          <div className="flex flex-col items-center gap-3 p-4 text-center">
            <p className="text-sm text-neutral-700" role="status">
              {message}
            </p>
            {phase !== "unavailable" ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => startRef.current(true)}
              >
                <RefreshCw aria-hidden />
                {t("auth.qr.refresh")}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
      <ol className="text-muted-foreground list-decimal space-y-1 pl-5 text-sm">
        <li>{t("auth.qr.step_open")}</li>
        <li>{t("auth.qr.step_scan")}</li>
        <li>{t("auth.qr.step_approve")}</li>
      </ol>
      <p className="text-muted-foreground text-center text-xs" role="status">
        {phase === "signing_in"
          ? t("auth.qr.signing_in")
          : t("auth.qr.auto_refresh")}
      </p>
    </div>
  );
}
