"use client";

import { Send, UserRound } from "lucide-react";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ExchangeForm } from "@/features/leads/lib/form-config";
import {
  publicLeadsService,
  type VisitorPostResult,
} from "@/features/leads/services/leads.service";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";

/*
 * The opened contact exchange forms. Split from contact-exchange.tsx and loaded on demand, so the
 * Select/form code is not part of the profile page's first load (TEC-858).
 */

// Mirrors backend leads/model limits.
const MAX_NAME = 150;
const MAX_EMAIL = 254;
const MAX_PHONE = 32;
const MAX_COMPANY = 150;
const MAX_TITLE = 150;

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const PHONE_RE = /^\+?[\d\s().-]+$/;

type FieldKey =
  "name" | "contact" | "email" | "phone" | "company" | "job_title" | "consent";

const ERROR_KEYS: Record<FieldKey, string> = {
  name: "leads.form.error.name_required",
  contact: "leads.form.error.contact_required",
  email: "leads.form.error.email_invalid",
  phone: "leads.form.error.phone_invalid",
  company: "leads.exchange.error.too_long",
  job_title: "leads.exchange.error.too_long",
  consent: "leads.form.error.consent_required",
};

export function ShareProfileForm({
  exchange,
  profiles,
  locale,
  onSent,
  onUseForm,
}: {
  exchange: ExchangeForm;
  profiles: ProfileSummary[];
  locale: string;
  onSent: () => void;
  onUseForm: () => void;
}) {
  const { t } = useLocale();
  const [profileUuid, setProfileUuid] = useState(
    () => (profiles.find((p) => p.is_primary) ?? profiles[0]).uuid,
  );
  const [consent, setConsent] = useState(false);
  const [consentError, setConsentError] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setConsentError(!consent);
    setFormError(null);
    if (!consent) return;
    setSending(true);
    try {
      const res = await publicLeadsService.shareProfile({
        path: exchange.path,
        via: exchange.via,
        profile_uuid: profileUuid,
        locale: locale === "en" ? "en" : "tr",
        consent: true,
        consent_version: exchange.consentVersion,
      });
      if (res.ok) onSent();
      else setFormError(t(shareErrorKey(res)));
    } catch {
      setFormError(t("leads.form.error_generic"));
    } finally {
      setSending(false);
    }
  };

  return (
    <form onSubmit={(e) => void submit(e)} noValidate className="space-y-3">
      <div className="grid gap-1.5">
        <Label htmlFor="exchange-profile">{t("leads.exchange.profile")}</Label>
        <Select value={profileUuid} onValueChange={setProfileUuid}>
          <SelectTrigger id="exchange-profile" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {profiles.map((p) => (
              <SelectItem key={p.uuid} value={p.uuid}>
                {profileLabel(p)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className="text-muted-foreground text-xs">
          {t("leads.exchange.profile_hint")}
        </p>
      </div>
      <ConsentField
        checked={consent}
        onChange={setConsent}
        invalid={consentError}
      />
      {formError ? (
        <p className="text-destructive text-sm">{formError}</p>
      ) : null}
      <Button type="submit" className="w-full" disabled={sending}>
        <UserRound className="size-4" />
        {sending ? t("leads.form.sending") : t("leads.exchange.share_profile")}
      </Button>
      <Button
        type="button"
        variant="link"
        size="sm"
        className="w-full"
        onClick={onUseForm}
      >
        {t("leads.exchange.use_form")}
      </Button>
    </form>
  );
}

export function ExchangeDetailsForm({
  exchange,
  locale,
  onSent,
  onUseProfile,
}: {
  exchange: ExchangeForm;
  locale: string;
  onSent: () => void;
  onUseProfile?: () => void;
}) {
  const { t } = useLocale();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [company, setCompany] = useState("");
  const [jobTitle, setJobTitle] = useState("");
  const [consent, setConsent] = useState(false);
  const [errors, setErrors] = useState<FieldKey[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);

  const validate = (): FieldKey[] => {
    const out: FieldKey[] = [];
    if (!name.trim()) out.push("name");
    if (!email.trim() && !phone.trim()) out.push("contact");
    if (email.trim() && !EMAIL_RE.test(email.trim())) out.push("email");
    const digits = phone.replace(/\D/g, "").length;
    if (
      phone.trim() &&
      (!PHONE_RE.test(phone.trim()) || digits < 7 || digits > 15)
    )
      out.push("phone");
    if (!consent) out.push("consent");
    return out;
  };

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const found = validate();
    setErrors(found);
    setFormError(null);
    if (found.length) return;
    setSending(true);
    try {
      const res = await publicLeadsService.exchange({
        path: exchange.path,
        via: exchange.via,
        name: name.trim(),
        email: email.trim() || undefined,
        phone: phone.trim() || undefined,
        company: company.trim() || undefined,
        job_title: jobTitle.trim() || undefined,
        locale: locale === "en" ? "en" : "tr",
        consent: true,
        consent_version: exchange.consentVersion,
      });
      if (res.ok) {
        onSent();
        return;
      }
      if (res.status === 429) setFormError(t("leads.form.error_rate_limited"));
      else {
        const mapped = serverFieldErrors(res.fields);
        if (mapped.length) setErrors(mapped);
        else setFormError(t("leads.form.error_generic"));
      }
    } catch {
      setFormError(t("leads.form.error_generic"));
    } finally {
      setSending(false);
    }
  };

  const has = (k: FieldKey) => errors.includes(k);
  const err = (...keys: FieldKey[]) => {
    const k = keys.find(has);
    return k ? (
      <p className="text-destructive text-xs">{t(ERROR_KEYS[k])}</p>
    ) : null;
  };

  return (
    <form onSubmit={(e) => void submit(e)} noValidate className="space-y-3">
      <div className="grid gap-1.5">
        <Label htmlFor="exchange-name">{t("leads.form.name")}</Label>
        <Input
          id="exchange-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={MAX_NAME}
          autoComplete="name"
          aria-invalid={has("name")}
        />
        {err("name")}
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="grid gap-1.5">
          <Label htmlFor="exchange-email">{t("leads.form.email")}</Label>
          <Input
            id="exchange-email"
            type="email"
            inputMode="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            maxLength={MAX_EMAIL}
            autoComplete="email"
            aria-invalid={has("email") || has("contact")}
          />
          {err("email")}
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="exchange-phone">{t("leads.form.phone")}</Label>
          <Input
            id="exchange-phone"
            type="tel"
            inputMode="tel"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            maxLength={MAX_PHONE}
            autoComplete="tel"
            aria-invalid={has("phone") || has("contact")}
          />
          {err("phone")}
        </div>
      </div>
      {has("contact") ? (
        err("contact")
      ) : (
        <p className="text-muted-foreground text-xs">
          {t("leads.form.contact_hint")}
        </p>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="grid gap-1.5">
          <Label htmlFor="exchange-company">
            {t("leads.exchange.company")}
          </Label>
          <Input
            id="exchange-company"
            value={company}
            onChange={(e) => setCompany(e.target.value)}
            maxLength={MAX_COMPANY}
            autoComplete="organization"
            aria-invalid={has("company")}
          />
          {err("company")}
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="exchange-title">
            {t("leads.exchange.job_title")}
          </Label>
          <Input
            id="exchange-title"
            value={jobTitle}
            onChange={(e) => setJobTitle(e.target.value)}
            maxLength={MAX_TITLE}
            autoComplete="organization-title"
            aria-invalid={has("job_title")}
          />
          {err("job_title")}
        </div>
      </div>
      <ConsentField
        checked={consent}
        onChange={setConsent}
        invalid={has("consent")}
      />
      {formError ? (
        <p className="text-destructive text-sm">{formError}</p>
      ) : null}
      <Button type="submit" className="w-full" disabled={sending}>
        <Send className="size-4" />
        {sending ? t("leads.form.sending") : t("leads.exchange.submit")}
      </Button>
      {onUseProfile ? (
        <Button
          type="button"
          variant="link"
          size="sm"
          className="w-full"
          onClick={onUseProfile}
        >
          {t("leads.exchange.use_profile")}
        </Button>
      ) : null}
    </form>
  );
}

function ConsentField({
  checked,
  onChange,
  invalid,
}: {
  checked: boolean;
  onChange: (on: boolean) => void;
  invalid: boolean;
}) {
  const { t } = useLocale();
  return (
    <>
      <label className="flex items-start gap-2 text-xs leading-relaxed">
        <Checkbox
          checked={checked}
          onCheckedChange={(on) => onChange(on === true)}
          aria-invalid={invalid}
          className="mt-0.5"
        />
        <span>{t("leads.exchange.consent_text")}</span>
      </label>
      {invalid ? (
        <p className="text-destructive text-xs">
          {t("leads.form.error.consent_required")}
        </p>
      ) : null}
    </>
  );
}

function profileLabel(p: ProfileSummary): string {
  const name = [p.first_name, p.last_name].filter(Boolean).join(" ");
  const sub = [p.title, p.company_name].filter(Boolean).join(", ");
  return sub ? `${name} — ${sub}` : name || p.username || p.uuid;
}

function shareErrorKey(res: Extract<VisitorPostResult, { ok: false }>): string {
  if (res.status === 429) return "leads.form.error_rate_limited";
  if (res.fields.some((f) => f.startsWith("email:email_or_phone_required")))
    return "leads.exchange.error.profile_no_contact";
  if (res.status === 409) return "leads.exchange.error.own_page";
  if (res.status === 403) return "leads.exchange.error.forbidden";
  return "leads.form.error_generic";
}

/** Maps API `field:code` details onto form fields. */
function serverFieldErrors(fields: string[]): FieldKey[] {
  const out = new Set<FieldKey>();
  for (const f of fields) {
    const [field, code] = f.split(":");
    if (field === "email" && code === "email_or_phone_required")
      out.add("contact");
    else if (
      field === "name" ||
      field === "email" ||
      field === "phone" ||
      field === "company" ||
      field === "job_title" ||
      field === "consent"
    ) {
      out.add(field);
    }
  }
  return [...out];
}
