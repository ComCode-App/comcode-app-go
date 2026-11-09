"use client";

import { ExternalLink } from "lucide-react";

import { OverviewSection } from "@/features/platform-overview/components/overview-section";
import { numberFormatter } from "@/features/platform-overview/lib/format";
import type { PlatformOverview } from "@/features/platform-overview/services/overview.service";
import { useLocale } from "@/providers/locale-provider";

/** The 10 most viewed profiles of the last 30 days. */
export function TopProfilesCard({ data, loading }: { data?: PlatformOverview; loading?: boolean }) {
  const { t, locale } = useLocale();
  const fmt = numberFormatter(locale);
  const profiles = data?.top_profiles ?? [];
  const max = Math.max(1, ...profiles.map((p) => p.views));

  return (
    <OverviewSection
      title={t("dashboard.top_profiles.title")}
      description={t("dashboard.top_profiles.description")}
      loading={loading}
    >
      {profiles.length === 0 ? (
        <p className="text-muted-foreground text-sm">{t("dashboard.top_profiles.empty")}</p>
      ) : (
        <ol className="space-y-3">
          {profiles.map((p, i) => {
            const name = p.name || p.company_name || t("dashboard.top_profiles.unnamed");
            return (
              <li key={p.uuid} className="grid grid-cols-[1.5rem_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1">
                <span className="text-muted-foreground text-xs tabular-nums">{i + 1}</span>
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{name}</p>
                  <p className="text-muted-foreground truncate text-xs">
                    {p.username ? (
                      <a
                        href={`/${p.username}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="hover:text-foreground inline-flex items-center gap-1"
                        aria-label={t("dashboard.top_profiles.open_public", { name })}
                      >
                        /{p.username}
                        <ExternalLink className="size-3" aria-hidden />
                      </a>
                    ) : (
                      p.company_name
                    )}
                  </p>
                </div>
                <div className="text-right">
                  <p className="text-sm font-semibold tabular-nums">
                    <span className="sr-only">{t("dashboard.top_profiles.views")}: </span>
                    {fmt.format(p.views)}
                  </p>
                  <p className="text-muted-foreground text-xs tabular-nums">
                    {t("dashboard.top_profiles.sessions", { count: fmt.format(p.unique_sessions) })}
                  </p>
                </div>
                <span className="bg-muted col-span-2 col-start-2 h-1.5 overflow-hidden rounded-full">
                  <span
                    className="block h-full rounded-full"
                    style={{ width: `${(p.views / max) * 100}%`, backgroundColor: "var(--chart-1)" }}
                  />
                </span>
              </li>
            );
          })}
        </ol>
      )}
    </OverviewSection>
  );
}
