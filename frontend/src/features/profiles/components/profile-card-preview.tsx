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
      (values?.addresses ?? []).filter(
        (a): a is Partial<AddressFormValues> =>
          Boolean(a?.city?.trim() || a?.street?.trim() || a?.country?.trim()),
      ),
    [values?.addresses],
  );

  return (
    <Card
      className={cn(
        "overflow-hidden border border-border shadow-sm bg-card transition-all",
        className,
      )}
    >
      {/* Cover Header */}
      <div className="relative h-28 sm:h-36 bg-gradient-to-r from-slate-900 via-indigo-950 to-slate-900 overflow-hidden">
        {values?.cover_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={values.cover_url}
            alt="Cover"
            className="w-full h-full object-cover opacity-80"
          />
        ) : (
          <div className="absolute inset-0 bg-[radial-gradient(#ffffff15_1px,transparent_1px)] [background-size:16px_16px] opacity-40" />
        )}

        {/* Locale Preview Selector Pills */}
        <div className="absolute top-3 right-3 flex items-center gap-1.5 bg-background/80 backdrop-blur-md rounded-full px-2 py-1 border border-border/50 text-[11px] font-medium shadow-sm">
          <Globe className="w-3.5 h-3.5 text-muted-foreground" />
          <button
            type="button"
            onClick={() => setSelectedPreviewLocale("tr")}
            className={cn(
              "px-1.5 py-0.5 rounded-full transition-colors",
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
              "px-1.5 py-0.5 rounded-full transition-colors",
              selectedPreviewLocale === "en"
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            EN
          </button>
        </div>
      </div>

      <CardContent className="pt-0 px-5 pb-5">
        {/* Avatar + Main Identity Info */}
        <div className="flex flex-col sm:flex-row items-start sm:items-end justify-between -mt-10 sm:-mt-12 mb-4 gap-3">
          <Avatar className="w-20 h-20 sm:w-24 sm:h-24 rounded-2xl border-4 border-background bg-muted shadow-md shrink-0">
            {values?.avatar_url ? (
              <AvatarImage
                src={values.avatar_url}
                alt={fullName}
                className="object-cover"
              />
            ) : null}
            <AvatarFallback className="rounded-2xl text-xl font-bold bg-primary/10 text-primary">
              {initials}
            </AvatarFallback>
          </Avatar>

          {values?.is_primary && (
            <Badge
              variant="outline"
              className="border-amber-500/40 text-amber-600 dark:text-amber-400 bg-amber-500/10 gap-1 text-xs font-normal"
            >
              <Star className="w-3 h-3 fill-current" />
              {t("profiles.preview.primary_card")}
            </Badge>
          )}
        </div>

        {/* Name & Title */}
        <div className="space-y-1 mb-4">
          <div className="flex items-center gap-2 flex-wrap">
            <h3 className="font-semibold text-lg text-foreground tracking-tight">
              {fullName || t("profiles.fields.name")}
            </h3>
            {activeTranslation?.nickname && (
              <span className="text-xs text-muted-foreground">
                ({activeTranslation.nickname})
              </span>
            )}
          </div>

          {(activeTranslation?.title || activeTranslation?.company_name) && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground flex-wrap">
              {activeTranslation.title && (
                <span className="font-medium text-foreground/80 flex items-center gap-1">
                  <Briefcase className="w-3 h-3 text-muted-foreground" />
                  {activeTranslation.title}
                </span>
              )}
              {activeTranslation.title && activeTranslation.company_name && (
                <span>•</span>
              )}
              {activeTranslation.company_name && (
                <span className="flex items-center gap-1">
                  <Building2 className="w-3 h-3 text-muted-foreground" />
                  {activeTranslation.company_name}
                </span>
              )}
            </div>
          )}
        </div>

        {/* Bio */}
        {activeTranslation?.bio && (
          <p className="text-xs text-muted-foreground line-clamp-3 mb-4 leading-relaxed bg-muted/30 p-2.5 rounded-lg border border-border/40">
            {activeTranslation.bio}
          </p>
        )}

        {/* Contacts Grid */}
        {contacts.length > 0 && (
          <div className="space-y-1.5 mb-4">
            <div className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
              {t("profiles.preview.contacts")}
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {contacts.map((c, i) => (
                <div
                  key={c.uuid || i}
                  className="flex items-center gap-2 p-2 rounded-lg bg-muted/40 border border-border/50 text-xs overflow-hidden"
                >
                  {c.category === "phone" && (
                    <Phone className="w-3.5 h-3.5 text-emerald-500 shrink-0" />
                  )}
                  {c.category === "email" && (
                    <Mail className="w-3.5 h-3.5 text-blue-500 shrink-0" />
                  )}
                  {c.category === "whatsapp" && (
                    <MessageCircle className="w-3.5 h-3.5 text-green-500 shrink-0" />
                  )}
                  {c.category === "url" && (
                    <Globe className="w-3.5 h-3.5 text-indigo-500 shrink-0" />
                  )}
                  {c.category === "other" && (
                    <User className="w-3.5 h-3.5 text-muted-foreground shrink-0" />
                  )}
                  <div className="flex-1 min-w-0">
                    <div className="truncate font-medium text-foreground">
                      {c.value}
                    </div>
                    <div className="text-[10px] text-muted-foreground capitalize">
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
          <div className="space-y-1.5 mb-4">
            <div className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
              {t("profiles.preview.online_social")}
            </div>
            <div className="flex flex-wrap gap-1.5">
              {activeSocials.map((s, i) => (
                <span
                  key={s.uuid || i}
                  className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-muted/60 border border-border/60 text-xs text-foreground font-medium"
                >
                  {s.icon_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={s.icon_url}
                      alt={s.platform || "Platform"}
                      className="w-3.5 h-3.5 object-contain shrink-0"
                    />
                  ) : (
                    <Globe className="w-3.5 h-3.5 text-muted-foreground" />
                  )}
                  <span className="capitalize">{s.label || s.platform}</span>
                </span>
              ))}
            </div>
          </div>
        )}

        {/* Addresses */}
        {addresses.length > 0 && (
          <div className="space-y-1.5 mb-4">
            <div className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
              {t("profiles.preview.location")}
            </div>
            {addresses.map((a, i) => (
              <div
                key={a.uuid || i}
                className="flex items-start gap-2 p-2 rounded-lg bg-muted/30 border border-border/40 text-xs text-muted-foreground"
              >
                <MapPin className="w-3.5 h-3.5 text-rose-500 shrink-0 mt-0.5" />
                <div className="leading-snug">
                  {a.label && (
                    <span className="font-semibold text-foreground mr-1">
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
        <div className="mt-5 pt-4 border-t border-border/60 space-y-1.5">
          <div className="flex items-center justify-between text-xs">
            <span className="text-muted-foreground flex items-center gap-1 font-medium">
              <Sparkles className="w-3.5 h-3.5 text-primary" />
              {t("profiles.preview.readiness")}
            </span>
            <span className="font-bold text-foreground">
              %{completionRate}
            </span>
          </div>
          <Progress value={completionRate} className="h-1.5" />
        </div>
      </CardContent>
    </Card>
  );
}
