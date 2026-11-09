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
  const canEdit =
    usePermission().can(permissions.profiles.write) && canManageAccount(user);
  const deleteProfile = useDeleteProfile();
  const setPrimary = useSetPrimaryProfile();

  const [userSelectedLocale, setUserSelectedLocale] = useState<
    "tr" | "en" | null
  >(null);

  const profileQuery = useQuery({
    queryKey: profilesKeys.detail(uuid),
    queryFn: () => profilesService.get(uuid),
    enabled: Boolean(uuid),
  });

  const profile = profileQuery.data;
  const selectedLocale = userSelectedLocale ?? profile?.primary_locale ?? "tr";

  // Selected translation details
  const activeTranslation = useMemo(() => {
    if (!profile?.translations || profile.translations.length === 0)
      return null;
    const found = profile.translations.find(
      (tr) => tr.locale === selectedLocale,
    );
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
    const filename =
      `${activeTranslation?.first_name || "contact"}_${activeTranslation?.last_name || ""}`.trim();
    await profilesService.downloadVCard(
      profile.uuid,
      filename,
      selectedLocale,
      version,
    );
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
        <div className="flex flex-wrap items-center gap-2">
          <ProfileCompletionBadge rate={profile.completion_rate} />

          <Button
            variant="outline"
            size="sm"
            onClick={() => router.push(routes.app.profiles.edit(profile.uuid))}
            className="gap-1.5"
          >
            <Pencil className="h-4 w-4" />
            <span>{t("profiles.actions.edit")}</span>
          </Button>

          <Button
            size="sm"
            onClick={() => handleDownloadVCard("4.0")}
            className="bg-primary text-primary-foreground gap-1.5 shadow-sm"
          >
            <Download className="h-4 w-4" />
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
              <Star className="h-4 w-4 fill-current" />
              <span>{t("profiles.actions.set_primary")}</span>
            </Button>
          )}

          <Button
            variant="ghost"
            size="sm"
            onClick={handleDelete}
            className="text-destructive hover:bg-destructive/10 h-8 px-2"
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Top Executive Header Card */}
        <Card className="border-border/80 bg-card overflow-hidden border shadow-sm">
          {/* Cover Banner */}
          <div className="relative h-40 overflow-hidden bg-gradient-to-r from-slate-900 via-indigo-950 to-slate-900 sm:h-56">
            {profile.cover_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={profile.cover_url}
                alt="Cover"
                className="h-full w-full object-cover opacity-85"
              />
            ) : (
              <div className="absolute inset-0 bg-[radial-gradient(#ffffff15_1px,transparent_1px)] [background-size:16px_16px] opacity-40" />
            )}

            {/* Language Switcher Bar on Top of Cover */}
            <div className="bg-background/85 border-border/60 absolute top-4 right-4 flex items-center gap-2 rounded-full border px-3 py-1.5 shadow-md backdrop-blur-md">
              <Languages className="text-primary h-4 w-4" />
              <div className="flex items-center gap-1 text-xs font-semibold">
                {availableLocales.map((loc) => (
                  <button
                    key={loc}
                    type="button"
                    onClick={() => setUserSelectedLocale(loc)}
                    className={cn(
                      "rounded-full px-2.5 py-1 uppercase transition-all",
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

          <CardContent className="px-6 pt-0 pb-6">
            <div className="-mt-12 mb-5 flex flex-col items-start justify-between gap-4 sm:-mt-16 sm:flex-row sm:items-end">
              <Avatar className="border-background bg-muted h-24 w-24 shrink-0 rounded-3xl border-4 shadow-xl sm:h-32 sm:w-32">
                {profile.avatar_url ? (
                  <AvatarImage
                    src={profile.avatar_url}
                    alt={fullName}
                    className="object-cover"
                  />
                ) : null}
                <AvatarFallback className="bg-primary/10 text-primary rounded-3xl text-2xl font-bold sm:text-3xl">
                  {initials}
                </AvatarFallback>
              </Avatar>

              <div className="flex flex-wrap items-center gap-2">
                {profile.is_primary && (
                  <Badge
                    variant="outline"
                    className="gap-1.5 border-amber-500/40 bg-amber-500/10 px-3 py-1 text-xs font-semibold text-amber-600 dark:text-amber-400"
                  >
                    <Star className="h-3.5 w-3.5 fill-current" />
                    {t("profiles.preview.primary_card")}
                  </Badge>
                )}
                <Badge
                  variant="outline"
                  className="px-2.5 py-1 font-mono text-xs"
                >
                  {selectedLocale.toUpperCase()}
                </Badge>
              </div>
            </div>

            {/* Identity Text Info */}
            <div className="space-y-2">
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="text-foreground text-2xl font-bold tracking-tight sm:text-3xl">
                  {fullName}
                  {profile.verification_status === "verified" ? (
                    <VerifiedBadge className="ml-2 text-[0.8em]" />
                  ) : null}
                </h1>
                {activeTranslation?.nickname && (
                  <span className="text-muted-foreground text-sm font-medium">
                    ({activeTranslation.nickname})
                  </span>
                )}
              </div>

              {(activeTranslation?.title ||
                activeTranslation?.company_name) && (
                <div className="text-muted-foreground flex flex-wrap items-center gap-3 text-sm">
                  {activeTranslation.title && (
                    <span className="text-foreground flex items-center gap-1.5 font-semibold">
                      <Briefcase className="text-primary h-4 w-4" />
                      {activeTranslation.title}
                    </span>
                  )}
                  {activeTranslation.title &&
                    activeTranslation.company_name && <span>•</span>}
                  {activeTranslation.company_name && (
                    <span className="flex items-center gap-1.5">
                      <Building2 className="text-muted-foreground h-4 w-4" />
                      {activeTranslation.company_name}
                    </span>
                  )}
                  {activeTranslation.department && (
                    <span className="bg-muted rounded-md px-2 py-0.5 text-xs">
                      {activeTranslation.department}
                    </span>
                  )}
                </div>
              )}

              {activeTranslation?.bio && (
                <p className="text-muted-foreground bg-muted/20 border-border/40 mt-3 max-w-3xl rounded-xl border p-3 text-sm leading-relaxed">
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
        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
          {/* Left Column: Direct Contacts & Addresses */}
          <div className="space-y-6 lg:col-span-6">
            {/* Direct Contacts Card */}
            <Card className="border-border shadow-sm">
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base">
                  <Phone className="h-4 w-4 text-emerald-500" />
                  {t("profiles.sections.contacts_title")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("profiles.sections.contacts_desc")}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                {!profile.contact_details ||
                profile.contact_details.length === 0 ? (
                  <p className="text-muted-foreground py-3 text-center text-xs italic">
                    {t("profiles.detail.no_contacts")}
                  </p>
                ) : (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    {profile.contact_details.map((c, idx) => {
                      const isPhone = c.category === "phone";
                      const isEmail = c.category === "email";
                      const isWhatsApp = c.category === "whatsapp";
                      const isUrl = c.category === "url";

                      let href = "#";
                      if (isPhone) href = `tel:${c.value.replace(/\s+/g, "")}`;
                      if (isEmail) href = `mailto:${c.value}`;
                      if (isWhatsApp)
                        href = `https://wa.me/${c.value.replace(/[^0-9]/g, "")}`;
                      if (isUrl)
                        href = c.value.startsWith("http")
                          ? c.value
                          : `https://${c.value}`;

                      return (
                        <a
                          key={c.uuid || idx}
                          href={href}
                          target={isUrl || isWhatsApp ? "_blank" : undefined}
                          rel="noreferrer"
                          className="bg-muted/30 border-border/60 hover:bg-muted/60 group flex items-center gap-3 overflow-hidden rounded-xl border p-3 transition-all"
                        >
                          <div className="bg-background border-border/60 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border shadow-xs">
                            {isPhone && (
                              <Phone className="h-4 w-4 text-emerald-500 transition-transform group-hover:scale-110" />
                            )}
                            {isEmail && (
                              <Mail className="h-4 w-4 text-blue-500 transition-transform group-hover:scale-110" />
                            )}
                            {isWhatsApp && (
                              <MessageCircle className="h-4 w-4 text-green-500 transition-transform group-hover:scale-110" />
                            )}
                            {isUrl && (
                              <Globe className="h-4 w-4 text-indigo-500 transition-transform group-hover:scale-110" />
                            )}
                            {!isPhone && !isEmail && !isWhatsApp && !isUrl && (
                              <User className="text-muted-foreground h-4 w-4" />
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="text-foreground truncate text-xs font-semibold">
                              {c.value}
                            </div>
                            <div className="text-muted-foreground flex items-center gap-1.5 text-[11px] capitalize">
                              {c.label ? (
                                <span className="text-primary font-medium">
                                  {c.label}
                                </span>
                              ) : (
                                <span>
                                  {t(`profiles.contact.category_${c.category}`)}
                                </span>
                              )}
                              {c.is_primary && (
                                <span className="bg-primary/10 text-primary rounded px-1 text-[9px] font-bold">
                                  {t("profiles.contact.is_primary")}
                                </span>
                              )}
                            </div>
                          </div>
                          <ExternalLink className="text-muted-foreground mr-1 h-3.5 w-3.5 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" />
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
                <CardTitle className="flex items-center gap-2 text-base">
                  <MapPin className="h-4 w-4 text-rose-500" />
                  {t("profiles.sections.addresses_title")}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                {!profile.addresses || profile.addresses.length === 0 ? (
                  <p className="text-muted-foreground py-2 text-center text-xs italic">
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
                        className="bg-muted/30 border-border/60 space-y-2 rounded-xl border p-3.5"
                      >
                        <div className="flex items-center justify-between">
                          <span className="text-foreground text-xs font-bold capitalize">
                            {a.label ||
                              t(`profiles.address.type_${a.type || "work"}`)}
                          </span>
                          {a.is_primary && (
                            <Badge
                              variant="secondary"
                              className="px-1.5 py-0 text-[9px]"
                            >
                              {t("profiles.address.is_primary")}
                            </Badge>
                          )}
                        </div>
                        <p className="text-muted-foreground text-xs leading-snug">
                          {fullAddress}
                        </p>
                        <div className="pt-1">
                          <a
                            href={mapsUrl}
                            target="_blank"
                            rel="noreferrer"
                            className="text-primary inline-flex items-center gap-1 text-[11px] font-medium hover:underline"
                          >
                            <span>{t("profiles.detail.maps_link")}</span>
                            <ExternalLink className="h-3 w-3" />
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
          <div className="space-y-6 lg:col-span-6">
            {/* Social & Web Profiles Card */}
            <Card className="border-border shadow-sm">
              <CardHeader className="pb-3">
                <CardTitle className="flex items-center gap-2 text-base">
                  <Share2 className="h-4 w-4 text-blue-500" />
                  {t("profiles.sections.social_title")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("profiles.sections.social_desc")}
                </CardDescription>
              </CardHeader>
              <CardContent>
                {!profile.social_links || profile.social_links.length === 0 ? (
                  <p className="text-muted-foreground py-3 text-center text-xs italic">
                    {t("profiles.detail.no_social")}
                  </p>
                ) : (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    {profile.social_links
                      .filter((s) => s.is_active)
                      .map((s, idx) => (
                        <a
                          key={s.uuid || idx}
                          href={s.url}
                          target="_blank"
                          rel="noreferrer"
                          className="bg-muted/30 border-border/60 hover:bg-muted/60 group flex items-center gap-3 overflow-hidden rounded-xl border p-3 transition-all"
                        >
                          <div className="bg-background border-border/60 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border shadow-xs">
                            {s.icon_url ? (
                              // eslint-disable-next-line @next/next/no-img-element
                              <img
                                src={s.icon_url}
                                alt={s.platform || "Platform"}
                                className="h-5 w-5 object-contain"
                              />
                            ) : (
                              <Globe className="text-muted-foreground h-5 w-5" />
                            )}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="text-foreground truncate text-xs font-semibold">
                              {s.label || s.platform}
                            </div>
                            <div className="text-muted-foreground truncate font-mono text-[11px]">
                              {s.url.replace(/^https?:\/\/(www\.)?/, "")}
                            </div>
                          </div>
                          <ExternalLink className="text-muted-foreground mr-1 h-3.5 w-3.5 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" />
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
                  <p className="text-muted-foreground text-xs leading-relaxed whitespace-pre-wrap">
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
