"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import type { PublicProfile } from "@/features/public-profile/lib/load";
import { trackProfileEvent } from "@/features/public-profile/lib/track";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

export type ProfileSibling = NonNullable<PublicProfile["siblings"]>[number];

/** Horizontal travel (px) that counts as a swipe. */
const SWIPE_MIN = 60;

type Props = {
  /** The person's open profiles in order, the shown one included (TEC-1326). */
  siblings: ProfileSibling[];
  /** The profile on screen. */
  current: string;
  /** Lift the indicator above a layout's own bottom bar (Comcode V1 tab bar). */
  raised?: boolean;
  children: ReactNode;
};

/**
 * Moves between a person's open profiles (TEC-1326). Each profile is its own page with its own
 * theme and layout, so moving is a navigation to /{username}: a finger swipe on touch screens,
 * arrow buttons on wider screens, and a dot with the profile's short name for each one.
 */
export function ProfileSwipe({ siblings, current, raised, children }: Props) {
  const { t } = useLocale();
  const router = useRouter();
  const index = Math.max(
    0,
    siblings.findIndex((s) => s.username === current),
  );
  const [drag, setDrag] = useState(0);
  const start = useRef<{ x: number; y: number; ignore: boolean } | null>(null);

  const prev = index > 0 ? siblings[index - 1] : undefined;
  const next = index < siblings.length - 1 ? siblings[index + 1] : undefined;

  useEffect(() => {
    if (prev) router.prefetch(`/${prev.username}`);
    if (next) router.prefetch(`/${next.username}`);
  }, [router, prev, next]);

  const go = (target?: ProfileSibling) => {
    if (!target || target.username === current) return;
    trackProfileEvent("profile.swipe", target.profile_uuid, {
      from: siblings[index]?.profile_uuid ?? "",
    });
    router.push(`/${encodeURIComponent(target.username)}`);
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement | null;
      if (el?.closest("input, textarea, select, [contenteditable=true]"))
        return;
      if (e.key === "ArrowLeft") go(prev);
      if (e.key === "ArrowRight") go(next);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  return (
    <div
      // Room for the indicator so it never covers the page's last line.
      className={cn("relative", raised ? "pb-28" : "pb-16")}
      onTouchStart={(e) => {
        const touch = e.touches[0];
        const el = e.target as HTMLElement;
        start.current = {
          x: touch.clientX,
          y: touch.clientY,
          // Form fields and dialogs keep their own gestures.
          ignore: Boolean(
            el.closest(
              "input, textarea, [role=dialog], [role=slider], [data-swipe-ignore]",
            ),
          ),
        };
      }}
      onTouchMove={(e) => {
        const s = start.current;
        if (!s || s.ignore) return;
        const dx = e.touches[0].clientX - s.x;
        const dy = e.touches[0].clientY - s.y;
        if (Math.abs(dx) > Math.abs(dy)) setDrag(dx);
      }}
      onTouchEnd={(e) => {
        const s = start.current;
        start.current = null;
        setDrag(0);
        if (!s || s.ignore) return;
        const touch = e.changedTouches[0];
        const dx = touch.clientX - s.x;
        const dy = touch.clientY - s.y;
        if (Math.abs(dx) < SWIPE_MIN || Math.abs(dx) < Math.abs(dy) * 1.5)
          return;
        go(dx < 0 ? next : prev);
      }}
      onTouchCancel={() => {
        start.current = null;
        setDrag(0);
      }}
    >
      <div
        className="motion-safe:transition-transform motion-safe:duration-200"
        style={
          drag
            ? {
                transform: `translateX(${Math.max(-80, Math.min(80, drag * 0.35))}px)`,
              }
            : undefined
        }
      >
        {children}
      </div>

      {prev ? (
        <Button
          type="button"
          size="icon"
          variant="secondary"
          aria-label={t("public_profile.swipe.previous", {
            name: prev.short_name,
          })}
          className="fixed top-1/2 left-3 z-30 hidden -translate-y-1/2 rounded-full bg-black/60 text-white shadow-lg hover:bg-black/75 md:inline-flex"
          onClick={() => go(prev)}
        >
          <ChevronLeft className="size-5" />
        </Button>
      ) : null}
      {next ? (
        <Button
          type="button"
          size="icon"
          variant="secondary"
          aria-label={t("public_profile.swipe.next", {
            name: next.short_name,
          })}
          className="fixed top-1/2 right-3 z-30 hidden -translate-y-1/2 rounded-full bg-black/60 text-white shadow-lg hover:bg-black/75 md:inline-flex"
          onClick={() => go(next)}
        >
          <ChevronRight className="size-5" />
        </Button>
      ) : null}

      <nav
        aria-label={t("public_profile.swipe.label")}
        className={cn(
          "fixed inset-x-0 z-30 flex justify-center px-4",
          raised
            ? "bottom-[calc(4.5rem+env(safe-area-inset-bottom))]"
            : "bottom-[max(1rem,env(safe-area-inset-bottom))]",
        )}
      >
        <ol className="flex max-w-full items-center gap-1 overflow-x-auto rounded-full bg-black/65 px-2 py-1.5 text-white shadow-lg backdrop-blur-sm">
          {siblings.map((s, i) => {
            const active = i === index;
            return (
              <li key={s.username} className="shrink-0">
                <button
                  type="button"
                  aria-current={active ? "page" : undefined}
                  onClick={() => go(s)}
                  className={cn(
                    "flex items-center gap-1.5 rounded-full px-2 py-1 text-xs font-medium focus-visible:ring-2 focus-visible:ring-white focus-visible:outline-none",
                    active ? "bg-white/20" : "text-white/80 hover:text-white",
                  )}
                >
                  <span
                    aria-hidden
                    className={cn(
                      "size-2 rounded-full",
                      active ? "bg-white" : "bg-white/50",
                    )}
                  />
                  <span
                    className={cn(
                      "max-w-[9rem] truncate",
                      !active && "max-sm:sr-only",
                    )}
                  >
                    {s.short_name}
                  </span>
                </button>
              </li>
            );
          })}
        </ol>
      </nav>
    </div>
  );
}
