"use client";

import { useState, useMemo } from "react";
import {
  Briefcase,
  Building2,
  Globe,
  Mail,
  MapPin,
  MessageCircle,
  Phone,
  Sparkles,
  Star,
  User,
} from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  calculateLiveCompletion,
  type ProfilePreviewData,
  type ProfileTranslationFormValues,
  type ContactDetailFormValues,
  type SocialLinkFormValues,
  type AddressFormValues,
} from "@/features/profiles/schemas/profile-form";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

interface ProfileCardPreviewProps {
  values: ProfilePreviewData;
  className?: string;
}

export function ProfileCardPreview({
  values,
  className,
}: ProfileCardPreviewProps) {
  const { t } = useLocale();
  const [selectedPreviewLocale, setSelectedPreviewLocale] = useState<
    "tr" | "en"
  >("tr");

  const completionRate = useMemo(
    () => calculateLiveCompletion(values),
    [values],
  );

  const activeTranslation = useMemo(() => {
    const list: Partial<ProfileTranslationFormValues>[] =
      values?.translations || [];
    const found = list.find((tr) => tr?.locale === selectedPreviewLocale);
    if (found) return found;
    return list[0];
  }, [values?.translations, selectedPreviewLocale]);

  const fullName = useMemo(() => {
    if (!activeTranslation) return "";
    const parts = [
      activeTranslation.prefix,
      activeTranslation.first_name,
      activeTranslation.middle_name,
      activeTranslation.last_name,
      activeTranslation.suffix,
    ].filter(Boolean);
    return parts.join(" ");
  }, [activeTranslation]);

  const initials = useMemo(() => {
    if (activeTranslation?.first_name || activeTranslation?.last_name) {
      const f = activeTranslation.first_name?.[0] || "";
      const l = activeTranslation.last_name?.[0] || "";
      return (f + l).toUpperCase() || "ID";
    }
    return "ID";
  }, [activeTranslation]);

  const activeSocials = useMemo(
    () =>
      (values?.social_links ?? []).filter(
        (s): s is Partial<SocialLinkFormValues> & { url: string } =>
          Boolean(s?.is_active && s?.url?.trim()),
      ),
    [values?.social_links],
  );

  const contacts = useMemo(
    () =>
      (values?.contact_details ?? []).filter(
        (c): c is Partial<ContactDetailFormValues> & { value: string } =>
          Boolean(c?.value?.trim()),
      ),
    [values?.contact_details],
  );

  const addresses = useMemo(
    () =>
      (values?.addresses ?? []).filter((a): a is Partial<AddressFormValues> =>
        Boolean(a?.city?.trim() || a?.street?.trim() || a?.country?.trim()),
      ),
    [values?.addresses],
  );

  return (
    <Card
      className={cn(
        "border-border bg-card overflow-hidden border shadow-sm transition-all",
        className,
      )}
    >
      {/* Cover Header */}
      <div className="relative h-28 overflow-hidden bg-gradient-to-r from-slate-900 via-indigo-950 to-slate-900 sm:h-36">
        {values?.cover_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={values.cover_url}
            alt="Cover"
            className="h-full w-full object-cover opacity-80"
          />
        ) : (
          <div className="absolute inset-0 bg-[radial-gradient(#ffffff15_1px,transparent_1px)] [background-size:16px_16px] opacity-40" />
        )}

        {/* Locale Preview Selector Pills */}
        <div className="bg-background/80 border-border/50 absolute top-3 right-3 flex items-center gap-1.5 rounded-full border px-2 py-1 text-[11px] font-medium shadow-sm backdrop-blur-md">
          <Globe className="text-muted-foreground h-3.5 w-3.5" />
          <button
            type="button"
            onClick={() => setSelectedPreviewLocale("tr")}
            className={cn(
              "rounded-full px-1.5 py-0.5 transition-colors",
              selectedPreviewLocale === "tr"
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            TR
          </button>
          <button
            type="button"
            onClick={() => setSelectedPreviewLocale("en")}
            className={cn(
              "rounded-full px-1.5 py-0.5 transition-colors",
              selectedPreviewLocale === "en"
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            EN
          </button>
        </div>
      </div>

      <CardContent className="px-5 pt-0 pb-5">
        {/* Avatar + Main Identity Info */}
        <div className="-mt-10 mb-4 flex flex-col items-start justify-between gap-3 sm:-mt-12 sm:flex-row sm:items-end">
          <Avatar className="border-background bg-muted h-20 w-20 shrink-0 rounded-2xl border-4 shadow-md sm:h-24 sm:w-24">
            {values?.avatar_url ? (
              <AvatarImage
                src={values.avatar_url}
                alt={fullName}
                className="object-cover"
              />
            ) : null}
            <AvatarFallback className="bg-primary/10 text-primary rounded-2xl text-xl font-bold">
              {initials}
            </AvatarFallback>
          </Avatar>

          {values?.is_primary && (
            <Badge
              variant="outline"
              className="gap-1 border-amber-500/40 bg-amber-500/10 text-xs font-normal text-amber-600 dark:text-amber-400"
            >
              <Star className="h-3 w-3 fill-current" />
              {t("profiles.preview.primary_card")}
            </Badge>
          )}
        </div>

        {/* Name & Title */}
        <div className="mb-4 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-foreground text-lg font-semibold tracking-tight">
              {fullName || t("profiles.fields.name")}
            </h3>
            {activeTranslation?.nickname && (
              <span className="text-muted-foreground text-xs">
                ({activeTranslation.nickname})
              </span>
            )}
          </div>

          {(activeTranslation?.title || activeTranslation?.company_name) && (
            <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
              {activeTranslation.title && (
                <span className="text-foreground/80 flex items-center gap-1 font-medium">
                  <Briefcase className="text-muted-foreground h-3 w-3" />
                  {activeTranslation.title}
                </span>
              )}
              {activeTranslation.title && activeTranslation.company_name && (
                <span>•</span>
              )}
              {activeTranslation.company_name && (
                <span className="flex items-center gap-1">
                  <Building2 className="text-muted-foreground h-3 w-3" />
                  {activeTranslation.company_name}
                </span>
              )}
            </div>
          )}
        </div>

        {/* Bio */}
        {activeTranslation?.bio && (
          <p className="text-muted-foreground bg-muted/30 border-border/40 mb-4 line-clamp-3 rounded-lg border p-2.5 text-xs leading-relaxed">
            {activeTranslation.bio}
          </p>
        )}

        {/* Contacts Grid */}
        {contacts.length > 0 && (
          <div className="mb-4 space-y-1.5">
            <div className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
              {t("profiles.preview.contacts")}
            </div>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {contacts.map((c, i) => (
                <div
                  key={c.uuid || i}
                  className="bg-muted/40 border-border/50 flex items-center gap-2 overflow-hidden rounded-lg border p-2 text-xs"
                >
                  {c.category === "phone" && (
                    <Phone className="h-3.5 w-3.5 shrink-0 text-emerald-500" />
                  )}
                  {c.category === "email" && (
                    <Mail className="h-3.5 w-3.5 shrink-0 text-blue-500" />
                  )}
                  {c.category === "whatsapp" && (
                    <MessageCircle className="h-3.5 w-3.5 shrink-0 text-green-500" />
                  )}
                  {c.category === "url" && (
                    <Globe className="h-3.5 w-3.5 shrink-0 text-indigo-500" />
                  )}
                  {c.category === "other" && (
                    <User className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="text-foreground truncate font-medium">
                      {c.value}
                    </div>
                    <div className="text-muted-foreground text-[10px] capitalize">
                      {c.label ? c.label : c.type || c.category}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Social Links */}
        {activeSocials.length > 0 && (
          <div className="mb-4 space-y-1.5">
            <div className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
              {t("profiles.preview.online_social")}
            </div>
            <div className="flex flex-wrap gap-1.5">
              {activeSocials.map((s, i) => (
                <span
                  key={s.uuid || i}
                  className="bg-muted/60 border-border/60 text-foreground inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium"
                >
                  {s.icon_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={s.icon_url}
                      alt={s.platform || "Platform"}
                      className="h-3.5 w-3.5 shrink-0 object-contain"
                    />
                  ) : (
                    <Globe className="text-muted-foreground h-3.5 w-3.5" />
                  )}
                  <span className="capitalize">{s.label || s.platform}</span>
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Addresses */}
        {addresses.length > 0 && (
          <div className="mb-4 space-y-1.5">
            <div className="text-muted-foreground text-[11px] font-medium tracking-wider uppercase">
              {t("profiles.preview.location")}
            </div>
            {addresses.map((a, i) => (
              <div
                key={a.uuid || i}
                className="bg-muted/30 border-border/40 text-muted-foreground flex items-start gap-2 rounded-lg border p-2 text-xs"
              >
                <MapPin className="mt-0.5 h-3.5 w-3.5 shrink-0 text-rose-500" />
                <div className="leading-snug">
                  {a.label && (
                    <span className="text-foreground mr-1 font-semibold">
                      {a.label}:
                    </span>
                  )}
                  {[a.street, a.extended, a.city, a.region, a.country]
                    .filter(Boolean)
                    .join(", ")}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Completion Rate Progress */}
        <div className="border-border/60 mt-5 space-y-1.5 border-t pt-4">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground flex items-center gap-1 font-medium">
              <Sparkles className="text-primary h-3.5 w-3.5" />
              {t("profiles.preview.readiness")}
            </span>
            <span className="text-foreground font-bold">%{completionRate}</span>
          </div>
          <Progress value={completionRate} className="h-1.5" />
        </div>
      </CardContent>
    </Card>
  );
}
