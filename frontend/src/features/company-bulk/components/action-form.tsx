"use client";

import { useQuery } from "@tanstack/react-query";
import { Plus, Trash2 } from "lucide-react";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { routes } from "@/config/routes";
import {
  companyBrandingKeys,
  companyBrandingService,
} from "@/features/company-branding/services/company-branding.service";
import {
  PremiumLockIcon,
  PremiumNotice,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import {
  LAYOUT_MODES,
  THEME_IDS,
  isPremiumTheme,
} from "@/features/public-profile/lib/themes";
import { useActiveSocialPlatformsQuery } from "@/features/social-platforms/hooks/use-social-platforms";
import { assertServiceMediaURL } from "@/lib/media/urls";
import { useLocale } from "@/providers/locale-provider";

import type {
  CompanyBulkAction,
  CompanyBulkParams,
} from "../services/company-bulk.service";

type Translate = (
  key: string,
  params?: Record<string, string | number>,
) => string;

/** A field the action may change: unchecked means "leave as is". */
type Optional = { enabled: boolean; value: string };

const KEEP = "__keep__";
const CLEAR = "__clear__";
const MAX_LINKS = 10;

export type AddressForm = {
  label: string;
  street: string;
  city: string;
  region: string;
  postal_code: string;
  country: string;
};

export type ActionFormState = {
  companyName: Optional;
  addressMode: "keep" | "custom" | "office";
  address: AddressForm;
  phone: Optional;
  email: Optional;
  website: Optional;
  /** Branding file the logo action points profiles at (TEC-946). */
  brandingSource: "logo" | "cover";
  targetField: "avatar" | "cover";
  theme: string;
  layout: string;
  links: { platform: string; label: string; url: string }[];
  replaceSamePlatform: boolean;
};

const off = (): Optional => ({ enabled: false, value: "" });

export function initialActionForm(): ActionFormState {
  return {
    companyName: off(),
    addressMode: "keep",
    address: {
      label: "",
      street: "",
      city: "",
      region: "",
      postal_code: "",
      country: "",
    },
    phone: off(),
    email: off(),
    website: off(),
    brandingSource: "logo",
    targetField: "avatar",
    theme: KEEP,
    layout: KEEP,
    links: [{ platform: "custom", label: "", url: "" }],
    replaceSamePlatform: true,
  };
}

const isWebURL = (v: string) => /^https?:\/\/[^\s/]+\.[^\s]+$/i.test(v.trim());

export type BuiltAction = {
  params: CompanyBulkParams;
  /** Human-readable changes for the preview step. */
  summary: string[];
  error: string | null;
};

/** Turns the form into API params, the preview lines and the first validation problem. */
export function buildAction(
  action: CompanyBulkAction,
  s: ActionFormState,
  t: Translate,
): BuiltAction {
  const summary: string[] = [];
  const valueOrCleared = (v: string) =>
    v.trim() ? v.trim() : t("company_bulk.preview.cleared");
  switch (action) {
    case "set_contact": {
      const params: Record<string, unknown> = {};
      if (s.companyName.enabled) {
        params.company_name = s.companyName.value.trim();
        summary.push(
          `${t("company_bulk.form.company_name")}: ${valueOrCleared(s.companyName.value)}`,
        );
      }
      if (s.addressMode === "office") {
        params.use_office_address = true;
        summary.push(t("company_bulk.preview.office_address"));
      } else if (s.addressMode === "custom") {
        const address = Object.fromEntries(
          Object.entries(s.address).map(([k, v]) => [k, v.trim()]),
        );
        params.address = address;
        const text = [address.street, address.city, address.country]
          .filter(Boolean)
          .join(", ");
        summary.push(
          `${t("company_bulk.form.address")}: ${text || t("company_bulk.preview.cleared")}`,
        );
      }
      for (const [key, field] of [
        ["phone", s.phone],
        ["email", s.email],
        ["website", s.website],
      ] as const) {
        if (!field.enabled) continue;
        params[key] = field.value.trim();
        summary.push(
          `${t(`company_bulk.form.${key}`)}: ${valueOrCleared(field.value)}`,
        );
      }
      let error: string | null = null;
      if (!summary.length) error = t("company_bulk.form.nothing_selected");
      else if (
        s.website.enabled &&
        s.website.value.trim() &&
        !isWebURL(s.website.value)
      )
        error = t("company_bulk.form.invalid_url");
      else if (
        s.email.enabled &&
        s.email.value.trim() &&
        !s.email.value.includes("@")
      )
        error = t("company_bulk.form.invalid_email");
      return { params: params as CompanyBulkParams, summary, error };
    }
    case "set_logo": {
      summary.push(
        t("company_bulk.preview.logo", {
          source: t(`company_bulk.form.branding_${s.brandingSource}`),
          target: t(`company_bulk.form.field_${s.targetField}`),
        }),
      );
      return {
        params: { source: s.brandingSource, target_field: s.targetField },
        summary,
        error: null,
      };
    }
    case "set_theme": {
      const params: Record<string, string> = {};
      if (s.theme !== KEEP) {
        params.theme = s.theme === CLEAR ? "" : s.theme;
        summary.push(
          `${t("company_bulk.form.theme")}: ${
            s.theme === CLEAR
              ? t("company_bulk.form.theme_inherit")
              : t(`profiles.public_page.theme_name.${s.theme}`)
          }`,
        );
      }
      if (s.layout !== KEEP) {
        params.layout_mode = s.layout === CLEAR ? "" : s.layout;
        summary.push(
          `${t("company_bulk.form.layout")}: ${
            s.layout === CLEAR
              ? t("company_bulk.form.layout_theme")
              : t(`profiles.public_page.layout_name.${s.layout}`)
          }`,
        );
      }
      return {
        params: params as CompanyBulkParams,
        summary,
        error: summary.length ? null : t("company_bulk.form.nothing_selected"),
      };
    }
    case "add_links": {
      const links = s.links
        .map((l) => ({
          platform: l.platform || "custom",
          label: l.label.trim(),
          url: l.url.trim(),
        }))
        .filter((l) => l.url);
      for (const l of links) summary.push(`${l.label || l.platform}: ${l.url}`);
      if (s.replaceSamePlatform)
        summary.push(t("company_bulk.preview.replace_same_platform"));
      let error: string | null = null;
      if (!links.length) error = t("company_bulk.form.links_required");
      else if (links.some((l) => !isWebURL(l.url)))
        error = t("company_bulk.form.invalid_url");
      return {
        params: { links, replace_same_platform: s.replaceSamePlatform },
        summary,
        error,
      };
    }
  }
}

type Props = {
  action: CompanyBulkAction;
  state: ActionFormState;
  onChange: (next: ActionFormState) => void;
};

export function ActionForm({ action, state, onChange }: Props) {
  const set = (patch: Partial<ActionFormState>) =>
    onChange({ ...state, ...patch });
  switch (action) {
    case "set_contact":
      return <ContactForm state={state} set={set} />;
    case "set_logo":
      return <LogoForm state={state} set={set} />;
    case "set_theme":
      return <ThemeForm state={state} set={set} />;
    case "add_links":
      return <LinksForm state={state} set={set} />;
  }
}

type SubProps = {
  state: ActionFormState;
  set: (patch: Partial<ActionFormState>) => void;
};

function OptionalField({
  id,
  label,
  field,
  onChange,
  placeholder,
  type = "text",
}: {
  id: string;
  label: string;
  field: Optional;
  onChange: (next: Optional) => void;
  placeholder?: string;
  type?: string;
}) {
  const { t } = useLocale();
  return (
    <div className="space-y-1.5">
      <div className="flex items-center gap-2">
        <Checkbox
          id={`${id}-enabled`}
          checked={field.enabled}
          onCheckedChange={(v) => onChange({ ...field, enabled: Boolean(v) })}
        />
        <Label htmlFor={`${id}-enabled`}>{label}</Label>
      </div>
      {field.enabled ? (
        <Input
          id={id}
          type={type}
          value={field.value}
          placeholder={placeholder ?? t("company_bulk.form.empty_clears")}
          onChange={(e) => onChange({ ...field, value: e.target.value })}
        />
      ) : null}
    </div>
  );
}

function ContactForm({ state, set }: SubProps) {
  const { t } = useLocale();
  const setAddress = (key: keyof AddressForm, value: string) =>
    set({ address: { ...state.address, [key]: value } });
  return (
    <div className="space-y-4">
      <p className="text-muted-foreground text-xs">
        {t("company_bulk.form.contact_hint")}
      </p>
      <OptionalField
        id="bulk-company-name"
        label={t("company_bulk.form.company_name")}
        field={state.companyName}
        onChange={(companyName) => set({ companyName })}
      />
      <div className="space-y-2">
        <Label>{t("company_bulk.form.address")}</Label>
        <RadioGroup
          value={state.addressMode}
          onValueChange={(v) =>
            set({ addressMode: v as ActionFormState["addressMode"] })
          }
          className="gap-2"
        >
          {(["keep", "office", "custom"] as const).map((mode) => (
            <div key={mode} className="flex items-center gap-2">
              <RadioGroupItem value={mode} id={`bulk-address-${mode}`} />
              <Label htmlFor={`bulk-address-${mode}`} className="font-normal">
                {t(`company_bulk.form.address_${mode}`)}
              </Label>
            </div>
          ))}
        </RadioGroup>
        {state.addressMode === "custom" ? (
          <div className="grid gap-2 sm:grid-cols-2">
            {(
              [
                "label",
                "street",
                "city",
                "region",
                "postal_code",
                "country",
              ] as const
            ).map((key) => (
              <Input
                key={key}
                aria-label={t(`company_bulk.form.address_${key}`)}
                placeholder={t(`company_bulk.form.address_${key}`)}
                value={state.address[key]}
                className={key === "street" ? "sm:col-span-2" : undefined}
                onChange={(e) => setAddress(key, e.target.value)}
              />
            ))}
          </div>
        ) : null}
      </div>
      <OptionalField
        id="bulk-phone"
        label={t("company_bulk.form.phone")}
        field={state.phone}
        onChange={(phone) => set({ phone })}
        type="tel"
      />
      <OptionalField
        id="bulk-email"
        label={t("company_bulk.form.email")}
        field={state.email}
        onChange={(email) => set({ email })}
        type="email"
      />
      <OptionalField
        id="bulk-website"
        label={t("company_bulk.form.website")}
        field={state.website}
        onChange={(website) => set({ website })}
        placeholder="https://"
        type="url"
      />
    </div>
  );
}

function LogoForm({ state, set }: SubProps) {
  const { t } = useLocale();
  const brandingQuery = useQuery({
    queryKey: companyBrandingKeys.branding,
    queryFn: companyBrandingService.getBranding,
    staleTime: 60_000,
  });
  const picked = brandingQuery.data?.[state.brandingSource];
  return (
    <div className="space-y-4">
      <p className="text-muted-foreground text-xs">
        {t("company_bulk.form.logo_hint")}
      </p>
      <div className="grid gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label>{t("company_bulk.form.branding_source")}</Label>
          <Select
            value={state.brandingSource}
            onValueChange={(v) =>
              set({ brandingSource: v as "logo" | "cover" })
            }
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="logo">
                {t("company_bulk.form.branding_logo")}
              </SelectItem>
              <SelectItem value="cover">
                {t("company_bulk.form.branding_cover")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>{t("company_bulk.form.target_field")}</Label>
          <Select
            value={state.targetField}
            onValueChange={(v) => set({ targetField: v as "avatar" | "cover" })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="avatar">
                {t("company_bulk.form.field_avatar")}
              </SelectItem>
              <SelectItem value="cover">
                {t("company_bulk.form.field_cover")}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      {picked ? (
        // eslint-disable-next-line @next/next/no-img-element -- API-owned media URL, any host
        <img
          src={assertServiceMediaURL(picked.url) ?? undefined}
          alt=""
          className="h-16 max-w-48 rounded border object-contain"
        />
      ) : brandingQuery.isSuccess ? (
        <p className="text-destructive text-xs">
          {t("company_bulk.form.branding_missing")}{" "}
          <Link href={routes.app.company.branding} className="underline">
            {t("company_bulk.form.branding_link")}
          </Link>
        </p>
      ) : null}
    </div>
  );
}

function ThemeForm({ state, set }: SubProps) {
  const { t } = useLocale();
  // Premium themes need design.premium (TEC-947); the API refuses the job otherwise.
  const allowPremium = useDesignPremium();
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <div className="space-y-1.5">
        <Label>{t("company_bulk.form.theme")}</Label>
        <Select value={state.theme} onValueChange={(theme) => set({ theme })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={KEEP}>{t("company_bulk.form.keep")}</SelectItem>
            <SelectItem value={CLEAR}>
              {t("company_bulk.form.theme_inherit")}
            </SelectItem>
            {THEME_IDS.map((id) => {
              const locked = !allowPremium && isPremiumTheme(id);
              return (
                <SelectItem key={id} value={id} disabled={locked}>
                  {t(`profiles.public_page.theme_name.${id}`)}
                  {locked ? <PremiumLockIcon /> : null}
                </SelectItem>
              );
            })}
          </SelectContent>
        </Select>
        {!allowPremium ? <PremiumNotice /> : null}
      </div>
      <div className="space-y-1.5">
        <Label>{t("company_bulk.form.layout")}</Label>
        <Select
          value={state.layout}
          onValueChange={(layout) => set({ layout })}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={KEEP}>{t("company_bulk.form.keep")}</SelectItem>
            <SelectItem value={CLEAR}>
              {t("company_bulk.form.layout_theme")}
            </SelectItem>
            {LAYOUT_MODES.map((id) => (
              <SelectItem key={id} value={id}>
                {t(`profiles.public_page.layout_name.${id}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </div>
  );
}

function LinksForm({ state, set }: SubProps) {
  const { t } = useLocale();
  const platformsQuery = useActiveSocialPlatformsQuery();
  const platforms = platformsQuery.data ?? [];
  const update = (
    i: number,
    patch: Partial<ActionFormState["links"][number]>,
  ) =>
    set({
      links: state.links.map((l, idx) => (idx === i ? { ...l, ...patch } : l)),
    });
  return (
    <div className="space-y-3">
      <p className="text-muted-foreground text-xs">
        {t("company_bulk.form.links_hint")}
      </p>
      {state.links.map((link, i) => (
        <div
          key={i}
          className="grid gap-2 rounded-md border p-2 sm:grid-cols-[10rem_1fr_auto]"
        >
          <Select
            value={link.platform}
            onValueChange={(platform) => update(i, { platform })}
          >
            <SelectTrigger aria-label={t("company_bulk.form.link_platform")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="custom">
                {t("company_bulk.form.link_custom")}
              </SelectItem>
              {platforms.map((p) => (
                <SelectItem key={p.slug} value={p.slug}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="grid gap-2 sm:grid-cols-2">
            <Input
              aria-label={t("company_bulk.form.link_label")}
              placeholder={t("company_bulk.form.link_label")}
              value={link.label}
              onChange={(e) => update(i, { label: e.target.value })}
            />
            <Input
              aria-label={t("company_bulk.form.link_url")}
              placeholder="https://"
              type="url"
              value={link.url}
              onChange={(e) => update(i, { url: e.target.value })}
            />
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label={t("company_bulk.form.link_remove")}
            disabled={state.links.length === 1}
            onClick={() =>
              set({ links: state.links.filter((_, idx) => idx !== i) })
            }
          >
            <Trash2 className="size-4" />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={state.links.length >= MAX_LINKS}
        onClick={() =>
          set({
            links: [...state.links, { platform: "custom", label: "", url: "" }],
          })
        }
      >
        <Plus className="me-1.5 size-3.5" />
        {t("company_bulk.form.link_add")}
      </Button>
      <div className="flex items-center gap-2">
        <Checkbox
          id="bulk-replace-platform"
          checked={state.replaceSamePlatform}
          onCheckedChange={(v) => set({ replaceSamePlatform: Boolean(v) })}
        />
        <Label htmlFor="bulk-replace-platform" className="font-normal">
          {t("company_bulk.form.replace_same_platform")}
        </Label>
      </div>
    </div>
  );
}
