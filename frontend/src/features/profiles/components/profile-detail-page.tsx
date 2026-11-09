"use client";

import { useState, useMemo } from "react";
import { useParams, useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Briefcase,
  Building2,
  Download,
  ExternalLink,
  Globe,
  Languages,
  Mail,
  MapPin,
  MessageCircle,
  Pencil,
  Phone,
  Share2,
  Star,
  Trash2,
  User,
} from "lucide-react";

import { ErrorState } from "@/components/common/error-state";
import { PageLoader } from "@/components/common/loading";
import { EntityPage } from "@/components/entity";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { ProfileCompletionBadge } from "@/features/profiles/components/profile-completion-badge";
import { ProfileCompletionChecklist } from "@/features/profiles/components/profile-completion-checklist";
import { ProfilePublicPageCard } from "@/features/profiles/components/profile-public-page-card";
import { ProfileVerificationCard } from "@/features/profiles/components/profile-verification-card";
import { VerifiedBadge } from "@/features/profiles/components/verified-badge";
import { profilesKeys } from "@/features/profiles/hooks/query-keys";
import {
  useDeleteProfile,
  useSetPrimaryProfile,
} from "@/features/profiles/hooks/use-profile-mutations";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { canManageAccount } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { usePermission } from "@/providers/permission-provider";
import { useLocale } from "@/providers/locale-provider";
import { cn } from "@/lib/utils";

export function ProfileDetailPage() {
  const { t } = useLocale();
  const router = useRouter();
  const params = useParams<{ uuid: string }>();
  const uuid = params?.uuid ?? "";

  const { confirmDelete } = useDialogs();
  const { user } = useAuth();
  const canEdit = usePermission().can(permissions.profiles.write) && canManageAccount(user);
  const deleteProfile = useDeleteProfile();
  const setPrimary = useSetPrimaryProfile();

  const [userSelectedLocale, setUserSelectedLocale] = useState<"tr" | "en" | null>(null);

  const profileQuery = useQuery({
    queryKey: profilesKeys.detail(uuid),
    queryFn: () => profilesService.get(uuid),
    enabled: Boolean(uuid),
  });

  const profile = profileQuery.data;
  const selectedLocale = userSelectedLocale ?? profile?.primary_locale ?? "tr";

  // Selected translation details
  const activeTranslation = useMemo(() => {
    if (!profile?.translations || profile.translations.length === 0) return null;
    const found = profile.translations.find((tr) => tr.locale === selectedLocale);
    return found || profile.translations[0];
  }, [profile, selectedLocale]);

  const fullName = useMemo(() => {
    if (!activeTranslation) return t("profiles.fields.name");
    const parts = [
      activeTranslation.prefix,
      activeTranslation.first_name,
      activeTranslation.middle_name,
      activeTranslation.last_name,
      activeTranslation.suffix,
    ].filter(Boolean);
    return parts.join(" ") || t("profiles.fields.name");
  }, [activeTranslation, t]);

  const initials = useMemo(() => {
    if (activeTranslation?.first_name || activeTranslation?.last_name) {
      const f = activeTranslation.first_name?.[0] || "";
      const l = activeTranslation.last_name?.[0] || "";
      return (f + l).toUpperCase() || "ID";
    }
    return "ID";
  }, [activeTranslation]);

  if (profileQuery.isLoading) {
    return <PageLoader label={t("common.loading")} />;
  }

  if (profileQuery.isError || !profile) {
    return (
      <ErrorState
        title={t("common.error")}
        description={profileQuery.error?.message || t("profiles.empty_title")}
        retryLabel={t("common.back")}
        onRetry={() => router.push(routes.app.profiles.root)}
      />
    );
  }

  const handleDownloadVCard = async (version: "4.0" = "4.0") => {
    const filename = `${activeTranslation?.first_name || "contact"}_${activeTranslation?.last_name || ""}`.trim();
    await profilesService.downloadVCard(profile.uuid, filename, selectedLocale, version);
  };

  const handleSetPrimary = async () => {
    await setPrimary.mutateAsync(profile.uuid);
  };

  const handleDelete = async () => {
    const confirmed = await confirmDelete({
      title: t("profiles.delete_title"),
      description: t("profiles.delete_description", { name: fullName }),
    });
    if (!confirmed) return;
    await deleteProfile.mutateAsync(profile.uuid);
    router.push(routes.app.profiles.root);
  };

  const availableLocales = (profile.translations || []).map((tr) => tr.locale);

  return (
    <EntityPage
      title={fullName}
      description={`${t("profiles.detail_title")} (${selectedLocale.toUpperCase()})`}
      permission={permissions.profiles.read}
      breadcrumbs={[
        { label: t("layout.breadcrumb_home"), href: routes.app.home },
        { label: t("profiles.title"), href: routes.app.profiles.root },
        { label: fullName },
      ]}
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <ProfileCompletionBadge rate={profile.completion_rate} />

          <Button
            variant="outline"
            size="sm"
            onClick={() => router.push(routes.app.profiles.edit(profile.uuid))}
            className="gap-1.5"
          >
            <Pencil className="w-4 h-4" />
            <span>{t("profiles.actions.edit")}</span>
          </Button>

          <Button
            size="sm"
            onClick={() => handleDownloadVCard("4.0")}
            className="gap-1.5 bg-primary text-primary-foreground shadow-sm"
          >
            <Download className="w-4 h-4" />
            <span>{t("profiles.actions.download_vcard")}</span>
          </Button>

          {!profile.is_primary && (
            <Button
              variant="outline"
              size="sm"
              onClick={handleSetPrimary}
              disabled={setPrimary.isPending}
              className="gap-1.5 text-amber-600 dark:text-amber-400"
            >
              <Star className="w-4 h-4 fill-current" />
              <span>{t("profiles.actions.set_primary")}</span>
            </Button>
          )}

          <Button
            variant="ghost"
            size="sm"
            onClick={handleDelete}
            className="text-destructive hover:bg-destructive/10 h-8 px-2"
          >
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Top Executive Header Card */}
        <Card className="overflow-hidden border border-border/80 shadow-sm bg-card">
          {/* Cover Banner */}
          <div className="relative h-40 sm:h-56 bg-gradient-to-r from-slate-900 via-indigo-950 to-slate-900 overflow-hidden">
            {profile.cover_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={profile.cover_url}
                alt="Cover"
                className="w-full h-full object-cover opacity-85"
              />
            ) : (
              <div className="absolute inset-0 bg-[radial-gradient(#ffffff15_1px,transparent_1px)] [background-size:16px_16px] opacity-40" />
            )}

            {/* Language Switcher Bar on Top of Cover */}
            <div className="absolute top-4 right-4 flex items-center gap-2 bg-background/85 backdrop-blur-md px-3 py-1.5 rounded-full border border-border/60 shadow-md">
              <Languages className="w-4 h-4 text-primary" />
              <div className="flex items-center gap-1 text-xs font-semibold">
                {availableLocales.map((loc) => (
                  <button
                    key={loc}
                    type="button"
                    onClick={() => setUserSelectedLocale(loc)}
                    className={cn(
                      "px-2.5 py-1 rounded-full uppercase transition-all",
                      selectedLocale === loc
                        ? "bg-primary text-primary-foreground shadow-xs"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                  >
                    {loc === "tr" ? "🇹🇷 TR" : "🇬🇧 EN"}
                  </button>
                ))}
              </div>
            </div>
          </div>

          <CardContent className="pt-0 px-6 pb-6">
            <div className="flex flex-col sm:flex-row items-start sm:items-end justify-between -mt-12 sm:-mt-16 mb-5 gap-4">
              <Avatar className="w-24 h-24 sm:w-32 sm:h-32 rounded-3xl border-4 border-background bg-muted shadow-xl shrink-0">
                {profile.avatar_url ? (
                  <AvatarImage
                    src={profile.avatar_url}
                    alt={fullName}
                    className="object-cover"
                  />
                ) : null}
                <AvatarFallback className="rounded-3xl text-2xl sm:text-3xl font-bold bg-primary/10 text-primary">
                  {initials}
                </AvatarFallback>
              </Avatar>

              <div className="flex items-center gap-2 flex-wrap">
                {profile.is_primary && (
                  <Badge
                    variant="outline"
                    className="border-amber-500/40 text-amber-600 dark:text-amber-400 bg-amber-500/10 gap-1.5 text-xs py-1 px-3 font-semibold"
                  >
                    <Star className="w-3.5 h-3.5 fill-current" />
                    {t("profiles.preview.primary_card")}
                  </Badge>
                )}
                <Badge variant="outline" className="text-xs font-mono py-1 px-2.5">
                  {selectedLocale.toUpperCase()}
                </Badge>
              </div>
            </div>

            {/* Identity Text Info */}
            <div className="space-y-2">
              <div className="flex items-center gap-3 flex-wrap">
                <h1 className="font-bold text-2xl sm:text-3xl text-foreground tracking-tight">
                  {fullName}
                  {profile.verification_status === "verified" ? (
                    <VerifiedBadge className="ml-2 text-[0.8em]" />
                  ) : null}
                </h1>
                {activeTranslation?.nickname && (
                  <span className="text-sm text-muted-foreground font-medium">
                    ({activeTranslation.nickname})
                  </span>
                )}
              </div>

              {(activeTranslation?.title || activeTranslation?.company_name) && (
                <div className="flex items-center gap-3 text-sm text-muted-foreground flex-wrap">
                  {activeTranslation.title && (
                    <span className="font-semibold text-foreground flex items-center gap-1.5">
                      <Briefcase className="w-4 h-4 text-primary" />
                      {activeTranslation.title}
                    </span>
                  )}
                  {activeTranslation.title && activeTranslation.company_name && (
                    <span>•</span>
                  )}
                  {activeTranslation.company_name && (
                    <span className="flex items-center gap-1.5">
                      <Building2 className="w-4 h-4 text-muted-foreground" />
                      {activeTranslation.company_name}
                    </span>
                  )}
                  {activeTranslation.department && (
                    <span className="text-xs bg-muted px-2 py-0.5 rounded-md">
                      {activeTranslation.department}
                    </span>
                  )}
                </div>
              )}

              {activeTranslation?.bio && (
                <p className="text-sm text-muted-foreground mt-3 leading-relaxed max-w-3xl bg-muted/20 p-3 rounded-xl border border-border/40">
                  {activeTranslation.bio}
                </p>
              )}
            </div>
          </CardContent>
        </Card>

        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-2">
          <ProfileCompletionChecklist
            profileUuid={profile.uuid}
            rate={profile.completion_rate}
            suggestions={profile.completion_suggestions}
            canEdit={canEdit}
          />
          <ProfileVerificationCard profileUuid={profile.uuid} />
        </div>

        <ProfilePublicPageCard profileUuid={profile.uuid} />

        {/* 2-Column Grid: Left (Contacts & Addresses) & Right (Social & Notes) */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 items-start">
          {/* Left Column: Direct Contacts & Addresses */}
          <div className="lg:col-span-6 space-y-6">
            {/* Direct Contacts Card */}
            <Card className="border-border shadow-sm">
              <CardHeader className="pb-3">
                <CardTitle className="text-base flex items-center gap-2">
                  <Phone className="w-4 h-4 text-emerald-500" />
                  {t("profiles.sections.contacts_title")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("profiles.sections.contacts_desc")}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {!profile.contact_details || profile.contact_details.length === 0 ? (
                  <p className="text-xs text-muted-foreground italic py-3 text-center">
                    {t("profiles.detail.no_contacts")}
                  </p>
                ) : (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    {profile.contact_details.map((c, idx) => {
                      const isPhone = c.category === "phone";
                      const isEmail = c.category === "email";
                      const isWhatsApp = c.category === "whatsapp";
                      const isUrl = c.category === "url";

                      let href = "#";
                      if (isPhone) href = `tel:${c.value.replace(/\s+/g, "")}`;
                      if (isEmail) href = `mailto:${c.value}`;
                      if (isWhatsApp) href = `https://wa.me/${c.value.replace(/[^0-9]/g, "")}`;
                      if (isUrl) href = c.value.startsWith("http") ? c.value : `https://${c.value}`;

                      return (
                        <a
                          key={c.uuid || idx}
                          href={href}
                          target={isUrl || isWhatsApp ? "_blank" : undefined}
                          rel="noreferrer"
                          className="flex items-center gap-3 p-3 rounded-xl bg-muted/30 border border-border/60 hover:bg-muted/60 transition-all group overflow-hidden"
                        >
                          <div className="w-10 h-10 rounded-xl bg-background border border-border/60 flex items-center justify-center shrink-0 shadow-xs">
                            {isPhone && (
                              <Phone className="w-4 h-4 text-emerald-500 group-hover:scale-110 transition-transform" />
                            )}
                            {isEmail && (
                              <Mail className="w-4 h-4 text-blue-500 group-hover:scale-110 transition-transform" />
                            )}
                            {isWhatsApp && (
                              <MessageCircle className="w-4 h-4 text-green-500 group-hover:scale-110 transition-transform" />
                            )}
                            {isUrl && (
                              <Globe className="w-4 h-4 text-indigo-500 group-hover:scale-110 transition-transform" />
                            )}
                            {!isPhone && !isEmail && !isWhatsApp && !isUrl && (
                              <User className="w-4 h-4 text-muted-foreground" />
                            )}
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="text-xs font-semibold text-foreground truncate">
                              {c.value}
                            </div>
                            <div className="text-[11px] text-muted-foreground capitalize flex items-center gap-1.5">
                              {c.label ? (
                                <span className="text-primary font-medium">
                                  {c.label}
                                </span>
                              ) : (
                                <span>{t(`profiles.contact.category_${c.category}`)}</span>
                              )}
                              {c.is_primary && (
                                <span className="text-[9px] bg-primary/10 text-primary px-1 rounded font-bold">
                                  {t("profiles.contact.is_primary")}
                                </span>
                              )}
                            </div>
                          </div>
                          <ExternalLink className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0 mr-1" />
                        </a>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Structured Locations (Addresses) */}
            <Card className="border-border shadow-sm">
              <CardHeader className="pb-3">
                <CardTitle className="text-base flex items-center gap-2">
                  <MapPin className="w-4 h-4 text-rose-500" />
                  {t("profiles.sections.addresses_title")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {!profile.addresses || profile.addresses.length === 0 ? (
                  <p className="text-xs text-muted-foreground italic py-2 text-center">
                    {t("profiles.detail.no_addresses")}
                  </p>
                ) : (
                  profile.addresses.map((a, idx) => {
                    const fullAddress = [
                      a.street,
                      a.extended,
                      a.city,
                      a.region,
                      a.postal_code,
                      a.country,
                    ]
                      .filter(Boolean)
                      .join(", ");
                    const mapsUrl = `https://maps.google.com/?q=${encodeURIComponent(fullAddress)}`;

                    return (
                      <div
                        key={a.uuid || idx}
                        className="p-3.5 rounded-xl bg-muted/30 border border-border/60 space-y-2"
                      >
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-bold capitalize text-foreground">
                            {a.label || t(`profiles.address.type_${a.type || "work"}`)}
                          </span>
                          {a.is_primary && (
                            <Badge
                              variant="secondary"
                              className="text-[9px] px-1.5 py-0"
                            >
                              {t("profiles.address.is_primary")}
                            </Badge>
                          )}
                        </div>
                        <p className="text-xs text-muted-foreground leading-snug">
                          {fullAddress}
                        </p>
                        <div className="pt-1">
                          <a
                            href={mapsUrl}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline"
                          >
                            <span>{t("profiles.detail.maps_link")}</span>
                            <ExternalLink className="w-3 h-3" />
                          </a>
                        </div>
                      </div>
                    );
                  })
                )}
              </CardContent>
            </Card>
          </div>

          {/* Right Column: Social Profiles & Notes */}
          <div className="lg:col-span-6 space-y-6">
            {/* Social & Web Profiles Card */}
            <Card className="border-border shadow-sm">
              <CardHeader className="pb-3">
                <CardTitle className="text-base flex items-center gap-2">
                  <Share2 className="w-4 h-4 text-blue-500" />
                  {t("profiles.sections.social_title")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("profiles.sections.social_desc")}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {!profile.social_links || profile.social_links.length === 0 ? (
                  <p className="text-xs text-muted-foreground italic py-3 text-center">
                    {t("profiles.detail.no_social")}
                  </p>
                ) : (
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    {profile.social_links
                      .filter((s) => s.is_active)
                      .map((s, idx) => (
                        <a
                          key={s.uuid || idx}
                          href={s.url}
                          target="_blank"
                          rel="noreferrer"
                          className="flex items-center gap-3 p-3 rounded-xl bg-muted/30 border border-border/60 hover:bg-muted/60 transition-all group overflow-hidden"
                        >
                          <div className="w-10 h-10 rounded-xl bg-background border border-border/60 flex items-center justify-center shrink-0 shadow-xs">
                            {s.icon_url ? (
                              // eslint-disable-next-line @next/next/no-img-element
                              <img
                                src={s.icon_url}
                                alt={s.platform || "Platform"}
                                className="w-5 h-5 object-contain"
                              />
                            ) : (
                              <Globe className="w-5 h-5 text-muted-foreground" />
                            )}
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="text-xs font-semibold text-foreground truncate">
                              {s.label || s.platform}
                            </div>
                            <div className="text-[11px] text-muted-foreground font-mono truncate">
                              {s.url.replace(/^https?:\/\/(www\.)?/, "")}
                            </div>
                          </div>
                          <ExternalLink className="w-3.5 h-3.5 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0 mr-1" />
                        </a>
                      ))}
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Note & Additional Info */}
            {activeTranslation?.note && (
              <Card className="border-border shadow-sm">
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm">
                    {t("profiles.fields.note")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <p className="text-xs text-muted-foreground leading-relaxed whitespace-pre-wrap">
                    {activeTranslation.note}
                  </p>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      </div>
    </EntityPage>
  );
}
