"use client";

import { useEffect, useRef, useState } from "react";

import { LeadCaptureForm } from "@/features/leads/components/lead-capture-form";
import type { LeadForm } from "@/features/leads/lib/form-config";
import { useLocale } from "@/providers/locale-provider";

import type { PublicBusinessPage } from "../services/business-page.service";
import { BusinessPageView } from "./business-page-view";

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

/** Fire-and-forget visitor event with the business page as subject (POST /v1/events via the BFF). */
function track(
  pageUuid: string,
  type: string,
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
        subject: { type: "business_page", id: pageUuid },
        session_id: sessionId(),
        properties: { ...properties, page: "business_page" },
      }),
    }).catch(() => undefined);
  } catch {
    // analytics must never break the page
  }
}

/**
 * Visitor page of a ComCode Business page (TEC-1327): the shared view plus the language switch,
 * the lead form behind a "lead_form" button and visitor analytics (`business.page.view`,
 * `business.button.click`, contact and social clicks, IBAN copies).
 */
export function BusinessPagePublic({
  page,
  lang,
  leadForm,
}: {
  page: PublicBusinessPage;
  lang?: string;
  leadForm: LeadForm | null;
}) {
  const { t } = useLocale();
  const locales: string[] = Array.from(
    new Set([page.primary_locale, ...Object.keys(page.translations)]),
  ).filter((l) => l === "tr" || l === "en");
  const [locale, setLocale] = useState<string>(
    lang && locales.includes(lang) ? lang : page.primary_locale,
  );
  const [formOpen, setFormOpen] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);

  const viewed = useRef(false);
  useEffect(() => {
    if (viewed.current) return;
    viewed.current = true;
    track(page.page_uuid, "business.page.view", { username: page.username });
  }, [page.page_uuid, page.username]);

  useEffect(() => {
    if (formOpen) formRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [formOpen]);

  const name =
    page.translations[locale]?.name ||
    page.translations[page.primary_locale]?.name ||
    page.company_name;

  return (
    <BusinessPageView
      page={page}
      locale={locale}
      locales={locales}
      onLocaleChange={setLocale}
      onLeadForm={leadForm ? () => setFormOpen(true) : undefined}
      onEvent={(type, props) => track(page.page_uuid, type, props)}
    >
      {leadForm && formOpen ? (
        <div
          ref={formRef}
          className="bg-card text-card-foreground rounded-3xl border p-5 shadow-sm"
        >
          <h2 className="text-muted-foreground mb-3 text-xs font-semibold tracking-wide uppercase">
            {t("leads.form.title")}
          </h2>
          <LeadCaptureForm
            path={leadForm.path}
            consentVersion={leadForm.consentVersion}
            ownerName={name}
            locale={locale}
          />
        </div>
      ) : null}
    </BusinessPageView>
  );
}
