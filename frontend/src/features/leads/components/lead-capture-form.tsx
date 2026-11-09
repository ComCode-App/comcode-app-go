"use client";

import { CheckCircle2, Send } from "lucide-react";
import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { publicLeadsService } from "@/features/leads/services/leads.service";
import { useLocale } from "@/providers/locale-provider";

// Mirrors backend leads/model limits.
const MAX_NAME = 150;
const MAX_EMAIL = 254;
const MAX_PHONE = 32;
const MAX_MESSAGE = 2000;

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const PHONE_RE = /^\+?[\d\s().-]+$/;

type Props = {
  /** Route the visitor came through: username, c/{code} or s/{slug}. */
  path: string;
  /** KVKK consent text version the API expects (LeadFormConfig.consent_version). */
  consentVersion: string;
  /** Card or smart link that opened the profile's own form; the lead is attributed to it (TEC-940). */
  via?: string;
  ownerName: string;
  locale: string;
};

type FieldKey = "name" | "contact" | "email" | "phone" | "message" | "consent";

const ERROR_KEYS: Record<FieldKey, string> = {
  name: "leads.form.error.name_required",
  contact: "leads.form.error.contact_required",
  email: "leads.form.error.email_invalid",
  phone: "leads.form.error.phone_invalid",
  message: "leads.form.error.message_too_long",
  consent: "leads.form.error.consent_required",
};

/** Visitor lead form on public pages (TEC-552). Posts to the same-origin BFF. */
export function LeadCaptureForm({ path, consentVersion, via, ownerName, locale }: Props) {
  const { t } = useLocale();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [message, setMessage] = useState("");
  const [consent, setConsent] = useState(false);
  const [errors, setErrors] = useState<FieldKey[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  const validate = (): FieldKey[] => {
    const out: FieldKey[] = [];
    if (!name.trim()) out.push("name");
    if (!email.trim() && !phone.trim()) out.push("contact");
    if (email.trim() && !EMAIL_RE.test(email.trim())) out.push("email");
    const digits = phone.replace(/\D/g, "").length;
    if (phone.trim() && (!PHONE_RE.test(phone.trim()) || digits < 7 || digits > 15)) out.push("phone");
    if (message.length > MAX_MESSAGE) out.push("message");
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
      const res = await publicLeadsService.submit({
        path,
        via,
        name: name.trim(),
        email: email.trim() || undefined,
        phone: phone.trim() || undefined,
        message: message.trim() || undefined,
        locale: locale === "en" ? "en" : "tr",
        consent: true,
        consent_version: consentVersion,
      });
      if (res.ok) {
        setSent(true);
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

  if (sent) {
    return (
      <div className="flex flex-col items-center gap-2 py-4 text-center">
        <CheckCircle2 className="size-8 text-emerald-600" />
        <p className="font-medium">{t("leads.form.success_title")}</p>
        <p className="text-muted-foreground text-sm">{t("leads.form.success_body")}</p>
      </div>
    );
  }

  const has = (k: FieldKey) => errors.includes(k);
  const err = (...keys: FieldKey[]) => {
    const k = keys.find(has);
    return k ? <p className="text-destructive text-xs">{t(ERROR_KEYS[k])}</p> : null;
  };

  return (
    <form onSubmit={(e) => void submit(e)} noValidate className="space-y-3">
      <p className="text-muted-foreground text-sm">{t("leads.form.subtitle", { name: ownerName })}</p>
      <div className="grid gap-1.5">
        <Label htmlFor="lead-name">{t("leads.form.name")}</Label>
        <Input
          id="lead-name"
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
          <Label htmlFor="lead-email">{t("leads.form.email")}</Label>
          <Input
            id="lead-email"
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
          <Label htmlFor="lead-phone">{t("leads.form.phone")}</Label>
          <Input
            id="lead-phone"
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
      {has("contact") ? err("contact") : <p className="text-muted-foreground text-xs">{t("leads.form.contact_hint")}</p>}
      <div className="grid gap-1.5">
        <Label htmlFor="lead-message">{t("leads.form.message")}</Label>
        <Textarea
          id="lead-message"
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          maxLength={MAX_MESSAGE}
          rows={3}
          aria-invalid={has("message")}
        />
        {err("message")}
      </div>
      <label className="flex items-start gap-2 text-xs leading-relaxed">
        <Checkbox
          checked={consent}
          onCheckedChange={(on) => setConsent(on === true)}
          aria-invalid={has("consent")}
          className="mt-0.5"
        />
        <span>{t("leads.form.consent_text")}</span>
      </label>
      {err("consent")}
      {formError ? <p className="text-destructive text-sm">{formError}</p> : null}
      <Button type="submit" className="w-full" disabled={sending}>
        <Send className="size-4" />
        {sending ? t("leads.form.sending") : t("leads.form.submit")}
      </Button>
    </form>
  );
}

/** Maps API `field:code` details onto form fields. */
function serverFieldErrors(fields: string[]): FieldKey[] {
  const out = new Set<FieldKey>();
  for (const f of fields) {
    const [field, code] = f.split(":");
    if (field === "email" && code === "email_or_phone_required") out.add("contact");
    else if (field === "name" || field === "email" || field === "phone" || field === "message" || field === "consent") {
      out.add(field);
    }
  }
  return [...out];
}
