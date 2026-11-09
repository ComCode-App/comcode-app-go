"use client";

import { useState, useMemo, useCallback } from "react";
import { useForm, useFieldArray, Controller, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  AlertCircle,
  Briefcase,
  Check,
  Globe,
  Image as ImageIcon,
  Languages,
  Loader2,
  MapPin,
  Phone,
  Plus,
  Share2,
  Sparkles,
  Star,
  Trash2,
  UploadCloud,
  User,
  X,
} from "lucide-react";
import { useRouter } from "next/navigation";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
import { DatePicker } from "@/components/ui/date-picker";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PhoneInput } from "@/components/ui/phone-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { routes } from "@/config/routes";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";
import { cn } from "@/lib/utils";

import {
  profileFormSchema,
  type ProfileFormValues,
  type ProfileTranslationFormValues,
} from "../schemas/profile-form";
import {
  type ProfileDetail,
  type ParsedVCardProfile,
  profilesService,
} from "../services/profiles.service";
import { useActiveSocialPlatformsQuery } from "@/features/social-platforms/hooks/use-social-platforms";
import { ProfileCardPreview } from "./profile-card-preview";
import {
  VCardImportDialog,
  type VCardApplyOptions,
} from "./vcard-import-dialog";
import { isApiError, suppressApiErrorToast } from "@/lib/api";

interface ProfileFormProps {
  initialData?: ProfileDetail;
  onSubmit: (values: ProfileFormValues) => Promise<void>;
  isLoading?: boolean;
  /** Editor tab to open, e.g. from a completion suggestion (`?tab=media`). */
  initialTab?: string | null;
}

type TabKey =
  "identity" | "media" | "contacts" | "social" | "addresses" | "settings";

const TAB_KEYS: readonly TabKey[] = [
  "identity",
  "media",
  "contacts",
  "social",
  "addresses",
  "settings",
];

function isTabKey(value: string | null | undefined): value is TabKey {
  return Boolean(value) && TAB_KEYS.includes(value as TabKey);
}

interface ValidationErrorItem {
  tab: TabKey;
  fieldId: string;
  message: string;
}

const activeDebounceTimers = new Map<number, NodeJS.Timeout>();

export function ProfileForm({
  initialData,
  onSubmit,
  isLoading,
  initialTab,
}: ProfileFormProps) {
  const { t } = useLocale();
  const router = useRouter();

  const [activeTab, setActiveTab] = useState<TabKey>(
    isTabKey(initialTab) ? initialTab : "identity",
  );
  // A new ?tab= (a suggestion clicked while the editor is open) switches the tab.
  const [requestedTab, setRequestedTab] = useState(initialTab);
  if (initialTab !== requestedTab) {
    setRequestedTab(initialTab);
    if (isTabKey(initialTab)) setActiveTab(initialTab);
  }
  const [activeLocale, setActiveLocale] = useState<"tr" | "en">(
    initialData?.primary_locale || "tr",
  );
  const [validationErrors, setValidationErrors] = useState<
    ValidationErrorItem[]
  >([]);

  // vCard import dialog state
  const [isVCardDialogOpen, setIsVCardDialogOpen] = useState(false);

  // Image upload states
  const [avatarUploading, setAvatarUploading] = useState(false);
  const [coverUploading, setCoverUploading] = useState(false);
  const [scrapingIndex, setScrapingIndex] = useState<number | null>(null);

  // Social platforms from platform admin catalog
  const { data: activePlatforms = [] } = useActiveSocialPlatformsQuery();

  const defaultTranslations: ProfileTranslationFormValues[] = useMemo(() => {
    if (initialData?.translations && initialData.translations.length > 0) {
      return initialData.translations.map((tr) => ({
        locale: tr.locale,
        prefix: tr.prefix || "",
        first_name: tr.first_name || "",
        last_name: tr.last_name || "",
        middle_name: tr.middle_name || "",
        suffix: tr.suffix || "",
        nickname: tr.nickname || "",
        company_name: tr.company_name || "",
        department: tr.department || "",
        title: tr.title || "",
        role: tr.role || "",
        bio: tr.bio || "",
        note: tr.note || "",
        is_default: tr.is_default || false,
      }));
    }
    return [
      {
        locale: "tr",
        prefix: "",
        first_name: "",
        last_name: "",
        middle_name: "",
        suffix: "",
        nickname: "",
        company_name: "",
        department: "",
        title: "",
        role: "",
        bio: "",
        note: "",
        is_default: true,
      },
    ];
  }, [initialData]);

  const form = useForm<ProfileFormValues>({
    resolver: zodResolver(profileFormSchema),
    defaultValues: {
      primary_locale: initialData?.primary_locale || "tr",
      birthday: initialData?.birthday || null,
      gender: initialData?.gender || "",
      avatar_url: initialData?.avatar_url || "",
      cover_url: initialData?.cover_url || "",
      is_primary: initialData?.is_primary || false,
      translations: defaultTranslations,
      contact_details: (initialData?.contact_details || []).map((c, idx) => ({
        uuid: c.uuid,
        category: c.category || "phone",
        type: c.type || "work",
        label: c.label || "",
        value: c.value || "",
        is_primary: c.is_primary ?? idx === 0,
        display_order: c.display_order || 0,
      })),
      addresses: (initialData?.addresses || []).map((a, idx) => ({
        uuid: a.uuid,
        type: a.type || "work",
        label: a.label || "",
        street: a.street || "",
        extended: a.extended || "",
        city: a.city || "",
        region: a.region || "",
        postal_code: a.postal_code || "",
        country: a.country || "",
        is_primary: a.is_primary ?? idx === 0,
        display_order: a.display_order || 0,
      })),
      social_links: (initialData?.social_links || []).map((s) => ({
        uuid: s.uuid,
        platform_id: s.platform_id || null,
        platform: s.platform || "general",
        label: s.label || "",
        url: s.url || "",
        icon_url: s.icon_url || "",
        display_order: s.display_order || 0,
        is_active: s.is_active ?? true,
      })),
    },
  });

  const debounceFetchMetadata = useCallback(
    (index: number, url: string) => {
      if (!url || !url.startsWith("http")) return;
      const existing = activeDebounceTimers.get(index);
      if (existing) {
        clearTimeout(existing);
      }
      const timer = setTimeout(async () => {
        try {
          setScrapingIndex(index);
          const meta = await profilesService.fetchLinkMetadata(url);
          if (meta.title && !form.getValues(`social_links.${index}.label`)) {
            form.setValue(`social_links.${index}.label`, meta.title, {
              shouldDirty: true,
            });
          }
          if (meta.icon_url) {
            form.setValue(`social_links.${index}.icon_url`, meta.icon_url, {
              shouldDirty: true,
            });
          }
        } catch {
          // Silently skip if auto-scraping fails during typing
        } finally {
          setScrapingIndex(null);
        }
      }, 600);
      activeDebounceTimers.set(index, timer);
    },
    [form],
  );

  const watchedValues = useWatch({ control: form.control });

  // Field Arrays
  const { fields: translationFields, append: appendTranslation } =
    useFieldArray({
      control: form.control,
      name: "translations",
    });

  const {
    fields: contactFields,
    append: appendContact,
    remove: removeContact,
  } = useFieldArray({
    control: form.control,
    name: "contact_details",
  });

  const {
    fields: addressFields,
    append: appendAddress,
    remove: removeAddress,
  } = useFieldArray({
    control: form.control,
    name: "addresses",
  });

  const {
    fields: socialFields,
    append: appendSocial,
    remove: removeSocial,
  } = useFieldArray({
    control: form.control,
    name: "social_links",
  });

  // Active translation index for current selected locale
  const currentTranslationIndex = useMemo(() => {
    const idx = translationFields.findIndex((f) => f.locale === activeLocale);
    return idx >= 0 ? idx : 0;
  }, [translationFields, activeLocale]);

  // Check if TR and EN exist
  const hasTr = useMemo(
    () => translationFields.some((f) => f.locale === "tr"),
    [translationFields],
  );
  const hasEn = useMemo(
    () => translationFields.some((f) => f.locale === "en"),
    [translationFields],
  );

  const handleApplyVCard = useCallback(
    (data: ParsedVCardProfile, options: VCardApplyOptions) => {
      const currentTranslation = form.getValues(
        `translations.${currentTranslationIndex}`,
      );

      // 1. Identity
      if (options.includeIdentity) {
        if (
          options.mergeMode === "replace" ||
          !currentTranslation?.first_name
        ) {
          form.setValue(
            `translations.${currentTranslationIndex}.first_name`,
            data.first_name || data.display_name || "",
            { shouldDirty: true },
          );
        }
        if (options.mergeMode === "replace" || !currentTranslation?.last_name) {
          form.setValue(
            `translations.${currentTranslationIndex}.last_name`,
            data.last_name || "",
            { shouldDirty: true },
          );
        }
        if (
          options.mergeMode === "replace" ||
          !currentTranslation?.middle_name
        ) {
          form.setValue(
            `translations.${currentTranslationIndex}.middle_name`,
            data.middle_name || "",
            { shouldDirty: true },
          );
        }
        if (options.mergeMode === "replace" || !currentTranslation?.prefix) {
          form.setValue(
            `translations.${currentTranslationIndex}.prefix`,
            data.prefix || "",
            { shouldDirty: true },
          );
        }
        if (options.mergeMode === "replace" || !currentTranslation?.suffix) {
          form.setValue(
            `translations.${currentTranslationIndex}.suffix`,
            data.suffix || "",
            { shouldDirty: true },
          );
        }
        if (options.mergeMode === "replace" || !currentTranslation?.nickname) {
          form.setValue(
            `translations.${currentTranslationIndex}.nickname`,
            data.nickname || "",
            { shouldDirty: true },
          );
        }
      }

      // 2. Work & Organization
      if (options.includeWork) {
        if (
          options.mergeMode === "replace" ||
          !currentTranslation?.company_name
        ) {
          form.setValue(
            `translations.${currentTranslationIndex}.company_name`,
            data.company_name || "",
            { shouldDirty: true },
          );
        }
        if (
          options.mergeMode === "replace" ||
          !currentTranslation?.department
        ) {
          form.setValue(
            `translations.${currentTranslationIndex}.department`,
            data.department || "",
            { shouldDirty: true },
          );
        }
        if (options.mergeMode === "replace" || !currentTranslation?.title) {
          form.setValue(
            `translations.${currentTranslationIndex}.title`,
            data.title || "",
            { shouldDirty: true },
          );
        }
        if (options.mergeMode === "replace" || !currentTranslation?.role) {
          form.setValue(
            `translations.${currentTranslationIndex}.role`,
            data.role || "",
            { shouldDirty: true },
          );
        }
      }

      // 3. Bio & Note
      if (options.includeBio) {
        if (options.mergeMode === "replace" || !currentTranslation?.bio) {
          if (data.bio) {
            form.setValue(
              `translations.${currentTranslationIndex}.bio`,
              data.bio,
              { shouldDirty: true },
            );
          }
        }
        if (options.mergeMode === "replace" || !currentTranslation?.note) {
          if (data.note) {
            form.setValue(
              `translations.${currentTranslationIndex}.note`,
              data.note,
              { shouldDirty: true },
            );
          }
        }
      }

      // 4. Avatar & Photo (Prefer S3 Public URL over DataURL)
      const resolvedAvatar = data.avatar_url || data.avatar_data_url;
      if (options.includeAvatar && resolvedAvatar) {
        if (options.mergeMode === "replace" || !form.getValues("avatar_url")) {
          form.setValue("avatar_url", resolvedAvatar, { shouldDirty: true });
        }
      }

      // 5. Birthday & Gender
      if (data.birthday) {
        if (options.mergeMode === "replace" || !form.getValues("birthday")) {
          form.setValue("birthday", data.birthday, { shouldDirty: true });
        }
      }
      if (data.gender) {
        if (options.mergeMode === "replace" || !form.getValues("gender")) {
          form.setValue("gender", data.gender, { shouldDirty: true });
        }
      }

      // 6. Contact Details (Phone & Email)
      if (
        options.includeContacts &&
        data.contact_details &&
        data.contact_details.length > 0
      ) {
        if (options.mergeMode === "replace") {
          form.setValue(
            "contact_details",
            data.contact_details.map((c, i) => ({
              category: c.category as
                "phone" | "email" | "url" | "whatsapp" | "other",
              type: c.type || "work",
              label: c.label || "",
              value: c.value,
              is_primary: c.is_primary ?? i === 0,
              display_order: i,
            })),
            { shouldDirty: true },
          );
        } else {
          const existing = form.getValues("contact_details") || [];
          const newItems = data.contact_details
            .filter((c) => !existing.some((ex) => ex.value === c.value))
            .map((c, i) => ({
              category: c.category as
                "phone" | "email" | "url" | "whatsapp" | "other",
              type: c.type || "work",
              label: c.label || "",
              value: c.value,
              is_primary: existing.length === 0 && i === 0,
              display_order: existing.length + i,
            }));
          if (newItems.length > 0) {
            form.setValue("contact_details", [...existing, ...newItems], {
              shouldDirty: true,
            });
          }
        }
      }

      // 7. Addresses
      if (
        options.includeAddresses &&
        data.addresses &&
        data.addresses.length > 0
      ) {
        if (options.mergeMode === "replace") {
          form.setValue(
            "addresses",
            data.addresses.map((a, i) => ({
              type: a.type || "work",
              label: a.label || "",
              street: a.street || "",
              extended: a.extended || "",
              city: a.city || "",
              region: a.region || "",
              postal_code: a.postal_code || "",
              country: a.country || "",
              is_primary: a.is_primary ?? i === 0,
              display_order: i,
            })),
            { shouldDirty: true },
          );
        } else {
          const existing = form.getValues("addresses") || [];
          const newItems = data.addresses.map((a, i) => ({
            type: a.type || "work",
            label: a.label || "",
            street: a.street || "",
            extended: a.extended || "",
            city: a.city || "",
            region: a.region || "",
            postal_code: a.postal_code || "",
            country: a.country || "",
            is_primary: existing.length === 0 && i === 0,
            display_order: existing.length + i,
          }));
          form.setValue("addresses", [...existing, ...newItems], {
            shouldDirty: true,
          });
        }
      }

      // 8. Social Links
      if (
        options.includeSocials &&
        data.social_links &&
        data.social_links.length > 0
      ) {
        if (options.mergeMode === "replace") {
          form.setValue(
            "social_links",
            data.social_links.map((s, i) => ({
              platform: s.platform || "general",
              platform_id: s.platform_id || null,
              label: s.label || "",
              url: s.url,
              icon_url: s.icon_url || "",
              display_order: i,
              is_active: true,
            })),
            { shouldDirty: true },
          );
        } else {
          const existing = form.getValues("social_links") || [];
          const newItems = data.social_links
            .filter((s) => !existing.some((ex) => ex.url === s.url))
            .map((s, i) => ({
              platform: s.platform || "general",
              platform_id: s.platform_id || null,
              label: s.label || "",
              url: s.url,
              icon_url: s.icon_url || "",
              display_order: existing.length + i,
              is_active: true,
            }));
          if (newItems.length > 0) {
            form.setValue("social_links", [...existing, ...newItems], {
              shouldDirty: true,
            });
          }
        }
      }
    },
    [currentTranslationIndex, form],
  );

  // Add / Preset-copy Locale
  const handleAddLocale = (targetLocale: "tr" | "en") => {
    const source = form.getValues(`translations.${currentTranslationIndex}`);
    appendTranslation({
      locale: targetLocale,
      prefix: source?.prefix || "",
      first_name: source?.first_name || "",
      last_name: source?.last_name || "",
      middle_name: source?.middle_name || "",
      suffix: source?.suffix || "",
      nickname: source?.nickname || "",
      company_name: source?.company_name || "",
      department: source?.department || "",
      title: source?.title || "",
      role: source?.role || "",
      bio: source?.bio || "",
      note: source?.note || "",
      is_default: false,
    });
    setActiveLocale(targetLocale);
    appToast.info(
      targetLocale === "en"
        ? t("profiles.toasts.locale_added_en")
        : t("profiles.toasts.locale_added_tr"),
    );
  };

  // Avatar Upload
  const handleAvatarFileChange = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (initialData?.uuid) {
      try {
        setAvatarUploading(true);
        const res = await profilesService.uploadAvatar(initialData.uuid, file);
        form.setValue("avatar_url", res.avatar_url, { shouldDirty: true });
        appToast.success(t("profiles.toasts.update_success"));
      } catch (err: unknown) {
        // API errors already get the global toast (TEC-1117).
        if (!isApiError(err)) {
          appToast.error(
            err instanceof Error ? err.message : t("common.error_generic"),
          );
        }
      } finally {
        setAvatarUploading(false);
      }
    } else {
      const localUrl = URL.createObjectURL(file);
      form.setValue("avatar_url", localUrl, { shouldDirty: true });
    }
  };

  // Cover Upload
  const handleCoverFileChange = async (
    e: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (initialData?.uuid) {
      try {
        setCoverUploading(true);
        const res = await profilesService.uploadCover(initialData.uuid, file);
        form.setValue("cover_url", res.cover_url, { shouldDirty: true });
        appToast.success(t("profiles.toasts.update_success"));
      } catch (err: unknown) {
        // API errors already get the global toast (TEC-1117).
        if (!isApiError(err)) {
          appToast.error(
            err instanceof Error ? err.message : t("common.error_generic"),
          );
        }
      } finally {
        setCoverUploading(false);
      }
    } else {
      const localUrl = URL.createObjectURL(file);
      form.setValue("cover_url", localUrl, { shouldDirty: true });
    }
  };

  // OpenGraph Link Metadata Fetcher
  const handleFetchLinkMetadata = async (index: number) => {
    const url = form.getValues(`social_links.${index}.url`);
    if (!url || !url.startsWith("http")) {
      appToast.error(
        t("profiles.validation.social_url_required", { index: index + 1 }),
      );
      return;
    }

    try {
      setScrapingIndex(index);
      const meta = await profilesService.fetchLinkMetadata(url);
      if (meta.title && !form.getValues(`social_links.${index}.label`)) {
        form.setValue(`social_links.${index}.label`, meta.title, {
          shouldDirty: true,
        });
      }
      if (meta.icon_url) {
        form.setValue(`social_links.${index}.icon_url`, meta.icon_url, {
          shouldDirty: true,
        });
      }
      appToast.success(t("profiles.toasts.metadata_fetched"));
    } catch (error) {
      // The metadata-specific text replaces the global API error toast (TEC-1117).
      suppressApiErrorToast(error);
      appToast.error(t("profiles.toasts.metadata_failed"));
    } finally {
      setScrapingIndex(null);
    }
  };

  // Handle Preset Platform Selection
  const handleSelectPlatformPreset = (index: number, platformSlug: string) => {
    const preset = activePlatforms.find((p) => p.slug === platformSlug);
    if (preset) {
      form.setValue(`social_links.${index}.platform`, preset.slug);
      form.setValue(`social_links.${index}.platform_id`, preset.id);
      form.setValue(`social_links.${index}.label`, preset.name);
      if (preset.icon_url) {
        form.setValue(`social_links.${index}.icon_url`, preset.icon_url);
      }
      const currentUrl = form.getValues(`social_links.${index}.url`);
      if (!currentUrl && preset.url_pattern) {
        form.setValue(
          `social_links.${index}.url`,
          preset.url_pattern.replace("{value}", ""),
        );
      }
    } else {
      form.setValue(`social_links.${index}.platform`, platformSlug);
    }
  };

  // Radio-like Primary Contact Selection
  const handleSelectPrimaryContact = (targetIdx: number) => {
    const contacts = form.getValues("contact_details") || [];
    contacts.forEach((_, idx) => {
      form.setValue(`contact_details.${idx}.is_primary`, idx === targetIdx, {
        shouldDirty: true,
      });
    });
  };

  // Radio-like Primary Address Selection
  const handleSelectPrimaryAddress = (targetIdx: number) => {
    const addresses = form.getValues("addresses") || [];
    addresses.forEach((_, idx) => {
      form.setValue(`addresses.${idx}.is_primary`, idx === targetIdx, {
        shouldDirty: true,
      });
    });
  };

  // Jump to field with error
  const handleJumpToError = (errorItem: ValidationErrorItem) => {
    setActiveTab(errorItem.tab);
    setTimeout(() => {
      const el = document.getElementById(errorItem.fieldId);
      if (el) {
        el.scrollIntoView({ behavior: "smooth", block: "center" });
        el.focus();
      }
    }, 100);
  };

  // Form Submit with Validation Summary
  const handleFormSubmit = async () => {
    const values = form.getValues();
    const errors: ValidationErrorItem[] = [];

    // 1. Identity validation
    const primaryLocale = values.primary_locale || "tr";
    const primaryTrans =
      values.translations.find((tr) => tr.locale === primaryLocale) ||
      values.translations[0];

    if (!primaryTrans?.first_name?.trim()) {
      errors.push({
        tab: "identity",
        fieldId: `field-first_name-${currentTranslationIndex}`,
        message: t("profiles.validation.first_name_required", {
          locale: primaryLocale.toUpperCase(),
        }),
      });
    }

    // 2. Contacts validation
    (values.contact_details || []).forEach((c, idx) => {
      if (!c.value?.trim()) {
        errors.push({
          tab: "contacts",
          fieldId: `field-contact-value-${idx}`,
          message: t("profiles.validation.contact_required", {
            index: idx + 1,
          }),
        });
      }
      if (c.category === "email" && c.value?.trim()) {
        const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        if (!emailRegex.test(c.value.trim())) {
          errors.push({
            tab: "contacts",
            fieldId: `field-contact-value-${idx}`,
            message: t("profiles.validation.email_invalid", {
              index: idx + 1,
            }),
          });
        }
      }
    });

    // 3. Social links validation
    (values.social_links || []).forEach((s, idx) => {
      if (!s.url?.trim()) {
        errors.push({
          tab: "social",
          fieldId: `field-social-url-${idx}`,
          message: t("profiles.validation.social_url_required", {
            index: idx + 1,
          }),
        });
      }
    });

    if (errors.length > 0) {
      setValidationErrors(errors);
      appToast.error(t("profiles.validation.resolve_prompt"));
      setActiveTab(errors[0].tab);
      return;
    }

    setValidationErrors([]);
    await onSubmit(values);
  };

  return (
    <div className="space-y-6">
      {/* VCard Import Autocomplete Dialog */}
      <VCardImportDialog
        open={isVCardDialogOpen}
        onOpenChange={setIsVCardDialogOpen}
        onApply={handleApplyVCard}
      />

      {/* Quick Action Top Bar: VCF Import & Autocomplete */}
      <div className="from-primary/10 via-primary/5 to-muted/40 border-primary/20 flex flex-col items-start justify-between gap-3 rounded-2xl border bg-gradient-to-r p-3.5 shadow-xs sm:flex-row sm:items-center">
        <div className="flex items-center gap-3">
          <div className="bg-primary/15 border-primary/30 text-primary flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border">
            <Sparkles className="h-4 w-4" />
          </div>
          <div className="space-y-0.5">
            <div className="text-foreground flex items-center gap-1.5 text-xs font-bold">
              <span>{t("profiles.vcard.banner_title")}</span>
              <Badge
                variant="outline"
                className="text-primary border-primary/30 px-1.5 py-0 font-mono text-[10px]"
              >
                .VCF / Apple / Google
              </Badge>
            </div>
            <p className="text-muted-foreground text-[11px]">
              {t("profiles.vcard.banner_desc")}
            </p>
          </div>
        </div>

        <Button
          type="button"
          variant="default"
          size="sm"
          onClick={() => setIsVCardDialogOpen(true)}
          className="w-full shrink-0 gap-1.5 rounded-xl text-xs font-semibold shadow-sm sm:w-auto"
        >
          <UploadCloud className="h-4 w-4" />
          <span>{t("profiles.vcard.banner_button")}</span>
        </Button>
      </div>

      {/* Top-Level Form Validation Summary Banner */}
      {validationErrors.length > 0 && (
        <Alert
          variant="destructive"
          className="border-destructive/40 bg-destructive/10 text-destructive animate-in fade-in slide-in-from-top-2 shadow-sm"
        >
          <AlertCircle className="h-5 w-5 shrink-0" />
          <div className="flex-1">
            <AlertTitle className="text-sm font-semibold">
              {t("profiles.validation.title", {
                count: validationErrors.length,
              })}
            </AlertTitle>
            <AlertDescription className="mt-1 space-y-1 text-xs">
              <p className="text-destructive/90">
                {t("profiles.validation.desc")}
              </p>
              <ul className="mt-2 list-inside list-disc space-y-0.5">
                {validationErrors.map((err, i) => (
                  <li key={i}>
                    <button
                      type="button"
                      onClick={() => handleJumpToError(err)}
                      className="hover:text-foreground cursor-pointer text-left font-medium underline transition-colors"
                    >
                      {err.message}
                    </button>
                  </li>
                ))}
              </ul>
            </AlertDescription>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setValidationErrors([])}
            className="text-destructive hover:bg-destructive/20 h-7 w-7 p-0"
          >
            <X className="h-4 w-4" />
          </Button>
        </Alert>
      )}

      {/* Main Grid: Form (Left) & Live Preview (Right) */}
      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
        {/* Left Column: Horizontally Scrollable Pill Tabs & Form Cards */}
        <div className="space-y-6 lg:col-span-7">
          {/* Responsive Horizontally Scrollable Pill Tabs */}
          <div className="no-scrollbar bg-muted/40 border-border/50 flex items-center gap-1.5 overflow-x-auto rounded-2xl border p-1.5 shadow-inner">
            <button
              type="button"
              onClick={() => setActiveTab("identity")}
              className={cn(
                "flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-semibold whitespace-nowrap transition-all",
                activeTab === "identity"
                  ? "bg-background text-foreground border-border/60 border shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50",
              )}
            >
              <User className="text-primary h-4 w-4" />
              <span>{t("profiles.tabs.identity")}</span>
              <Badge
                variant="secondary"
                className="px-1.5 py-0 font-mono text-[10px] uppercase"
              >
                {translationFields.length}
              </Badge>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("media")}
              className={cn(
                "flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-semibold whitespace-nowrap transition-all",
                activeTab === "media"
                  ? "bg-background text-foreground border-border/60 border shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50",
              )}
            >
              <ImageIcon className="h-4 w-4 text-indigo-500" />
              <span>{t("profiles.tabs.media")}</span>
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("contacts")}
              className={cn(
                "flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-semibold whitespace-nowrap transition-all",
                activeTab === "contacts"
                  ? "bg-background text-foreground border-border/60 border shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50",
              )}
            >
              <Phone className="h-4 w-4 text-emerald-500" />
              <span>{t("profiles.tabs.contacts")}</span>
              {contactFields.length > 0 && (
                <Badge
                  variant="secondary"
                  className="px-1.5 py-0 font-mono text-[10px]"
                >
                  {contactFields.length}
                </Badge>
              )}
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("social")}
              className={cn(
                "flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-semibold whitespace-nowrap transition-all",
                activeTab === "social"
                  ? "bg-background text-foreground border-border/60 border shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50",
              )}
            >
              <Share2 className="h-4 w-4 text-blue-500" />
              <span>{t("profiles.tabs.social")}</span>
              {socialFields.length > 0 && (
                <Badge
                  variant="secondary"
                  className="px-1.5 py-0 font-mono text-[10px]"
                >
                  {socialFields.length}
                </Badge>
              )}
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("addresses")}
              className={cn(
                "flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-semibold whitespace-nowrap transition-all",
                activeTab === "addresses"
                  ? "bg-background text-foreground border-border/60 border shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50",
              )}
            >
              <MapPin className="h-4 w-4 text-rose-500" />
              <span>{t("profiles.tabs.addresses")}</span>
              {addressFields.length > 0 && (
                <Badge
                  variant="secondary"
                  className="px-1.5 py-0 font-mono text-[10px]"
                >
                  {addressFields.length}
                </Badge>
              )}
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("settings")}
              className={cn(
                "flex shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-xs font-semibold whitespace-nowrap transition-all",
                activeTab === "settings"
                  ? "bg-background text-foreground border-border/60 border shadow-sm"
                  : "text-muted-foreground hover:text-foreground hover:bg-muted/50",
              )}
            >
              <Sparkles className="h-4 w-4 text-amber-500" />
              <span>{t("profiles.tabs.settings")}</span>
            </button>
          </div>

          {/* Tab 1: Identity & Multi-Locale Translations */}
          {activeTab === "identity" && (
            <Card className="border-border shadow-sm">
              <CardHeader className="pb-4">
                <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
                  <div>
                    <CardTitle className="flex items-center gap-2 text-base">
                      <Languages className="text-primary h-4 w-4" />
                      {t("profiles.sections.multi_locale_title")}
                    </CardTitle>
                    <CardDescription className="text-xs">
                      {t("profiles.sections.multi_locale_desc")}
                    </CardDescription>
                  </div>

                  {/* Multi-Language Switcher Pills */}
                  <div className="bg-muted/60 border-border/50 flex items-center gap-1 rounded-xl border p-1">
                    {hasTr && (
                      <button
                        type="button"
                        onClick={() => setActiveLocale("tr")}
                        className={cn(
                          "flex items-center gap-1.5 rounded-lg px-3 py-1 text-xs font-semibold transition-all",
                          activeLocale === "tr"
                            ? "bg-background text-foreground shadow-sm"
                            : "text-muted-foreground hover:text-foreground",
                        )}
                      >
                        <span>🇹🇷 Türkçe (TR)</span>
                        {watchedValues.primary_locale === "tr" && (
                          <Badge
                            variant="secondary"
                            className="px-1 py-0 text-[9px] uppercase"
                          >
                            {t("profiles.preview.primary_card")}
                          </Badge>
                        )}
                      </button>
                    )}

                    {hasEn && (
                      <button
                        type="button"
                        onClick={() => setActiveLocale("en")}
                        className={cn(
                          "flex items-center gap-1.5 rounded-lg px-3 py-1 text-xs font-semibold transition-all",
                          activeLocale === "en"
                            ? "bg-background text-foreground shadow-sm"
                            : "text-muted-foreground hover:text-foreground",
                        )}
                      >
                        <span>🇬🇧 English (EN)</span>
                        {watchedValues.primary_locale === "en" && (
                          <Badge
                            variant="secondary"
                            className="px-1 py-0 text-[9px] uppercase"
                          >
                            {t("profiles.preview.primary_card")}
                          </Badge>
                        )}
                      </button>
                    )}

                    {!hasEn && (
                      <button
                        type="button"
                        onClick={() => handleAddLocale("en")}
                        className="text-primary hover:bg-primary/10 flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-medium transition-colors"
                      >
                        <Plus className="h-3.5 w-3.5" />
                        <span>{t("profiles.actions.add_english")}</span>
                      </button>
                    )}

                    {!hasTr && (
                      <button
                        type="button"
                        onClick={() => handleAddLocale("tr")}
                        className="text-primary hover:bg-primary/10 flex items-center gap-1 rounded-lg px-2.5 py-1 text-xs font-medium transition-colors"
                      >
                        <Plus className="h-3.5 w-3.5" />
                        <span>{t("profiles.actions.add_turkish")}</span>
                      </button>
                    )}
                  </div>
                </div>
              </CardHeader>

              <CardContent className="space-y-4 pt-0">
                {/* Name Fields */}
                <div className="bg-muted/20 border-border/50 space-y-4 rounded-xl border p-4">
                  <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-semibold tracking-wider uppercase">
                    <User className="h-3.5 w-3.5" />
                    <span>
                      {t("profiles.sections.names")} (
                      {activeLocale.toUpperCase()})
                    </span>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-12">
                    <div className="sm:col-span-2">
                      <Label className="text-xs">
                        {t("profiles.fields.prefix")}
                      </Label>
                      <Input
                        placeholder={t("profiles.fields.prefix_placeholder")}
                        {...form.register(
                          `translations.${currentTranslationIndex}.prefix`,
                        )}
                      />
                    </div>
                    <div className="sm:col-span-5">
                      <Label className="text-xs">
                        {t("profiles.fields.first_name")}{" "}
                        <span className="text-destructive">*</span>
                      </Label>
                      <Input
                        id={`field-first_name-${currentTranslationIndex}`}
                        placeholder={t(
                          "profiles.fields.first_name_placeholder",
                        )}
                        {...form.register(
                          `translations.${currentTranslationIndex}.first_name`,
                        )}
                      />
                    </div>
                    <div className="sm:col-span-5">
                      <Label className="text-xs">
                        {t("profiles.fields.last_name")}
                      </Label>
                      <Input
                        placeholder={t("profiles.fields.last_name_placeholder")}
                        {...form.register(
                          `translations.${currentTranslationIndex}.last_name`,
                        )}
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.middle_name")}
                      </Label>
                      <Input
                        placeholder={t(
                          "profiles.fields.middle_name_placeholder",
                        )}
                        {...form.register(
                          `translations.${currentTranslationIndex}.middle_name`,
                        )}
                      />
                    </div>
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.suffix")}
                      </Label>
                      <Input
                        placeholder={t("profiles.fields.suffix_placeholder")}
                        {...form.register(
                          `translations.${currentTranslationIndex}.suffix`,
                        )}
                      />
                    </div>
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.nickname")}
                      </Label>
                      <Input
                        placeholder={t("profiles.fields.nickname_placeholder")}
                        {...form.register(
                          `translations.${currentTranslationIndex}.nickname`,
                        )}
                      />
                    </div>
                  </div>
                </div>

                {/* Professional Info */}
                <div className="bg-muted/20 border-border/50 space-y-4 rounded-xl border p-4">
                  <div className="text-muted-foreground flex items-center gap-1.5 text-xs font-semibold tracking-wider uppercase">
                    <Briefcase className="h-3.5 w-3.5" />
                    <span>{t("profiles.sections.work")}</span>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.company_name")}
                      </Label>
                      <Input
                        placeholder={t(
                          "profiles.fields.company_name_placeholder",
                        )}
                        {...form.register(
                          `translations.${currentTranslationIndex}.company_name`,
                        )}
                      />
                    </div>
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.title")}
                      </Label>
                      <Input
                        placeholder={t("profiles.fields.title_placeholder")}
                        {...form.register(
                          `translations.${currentTranslationIndex}.title`,
                        )}
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.department")}
                      </Label>
                      <Input
                        placeholder={t(
                          "profiles.fields.department_placeholder",
                        )}
                        {...form.register(
                          `translations.${currentTranslationIndex}.department`,
                        )}
                      />
                    </div>
                    <div>
                      <Label className="text-xs">
                        {t("profiles.fields.role")}
                      </Label>
                      <Input
                        placeholder={t("profiles.fields.role_placeholder")}
                        {...form.register(
                          `translations.${currentTranslationIndex}.role`,
                        )}
                      />
                    </div>
                  </div>
                </div>

                {/* Bio & Note */}
                <div className="bg-muted/20 border-border/50 space-y-3 rounded-xl border p-4">
                  <div className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
                    {t("profiles.sections.bio_note")}
                  </div>
                  <div>
                    <Label className="text-xs">
                      {t("profiles.fields.bio")}
                    </Label>
                    <Textarea
                      placeholder={t("profiles.fields.bio_placeholder")}
                      rows={3}
                      {...form.register(
                        `translations.${currentTranslationIndex}.bio`,
                      )}
                    />
                  </div>
                  <div>
                    <Label className="text-xs">
                      {t("profiles.fields.note")}
                    </Label>
                    <Textarea
                      placeholder={t("profiles.fields.note_placeholder")}
                      rows={2}
                      {...form.register(
                        `translations.${currentTranslationIndex}.note`,
                      )}
                    />
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Tab 2: Media & Photo Uploaders */}
          {activeTab === "media" && (
            <Card className="border-border shadow-sm">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <ImageIcon className="h-4 w-4 text-indigo-500" />
                  {t("profiles.sections.media_title")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("profiles.sections.media_desc")}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-6">
                {/* Avatar Uploader Dropzone */}
                <div className="border-border bg-muted/20 flex flex-col items-center gap-5 rounded-xl border border-dashed p-4 sm:flex-row">
                  <Avatar className="border-border bg-muted h-24 w-24 shrink-0 rounded-2xl border-2 shadow-sm">
                    {watchedValues.avatar_url ? (
                      <AvatarImage
                        src={watchedValues.avatar_url}
                        alt="Avatar"
                        className="object-cover"
                      />
                    ) : null}
                    <AvatarFallback className="rounded-2xl text-xl font-bold">
                      <User className="text-muted-foreground h-8 w-8" />
                    </AvatarFallback>
                  </Avatar>

                  <div className="flex-1 space-y-2 text-center sm:text-left">
                    <div className="text-sm font-semibold">
                      {t("profiles.fields.avatar")}
                    </div>
                    <p className="text-muted-foreground text-xs">
                      {t("profiles.fields.avatar_hint")}
                    </p>
                    <div className="flex items-center justify-center gap-2 pt-1 sm:justify-start">
                      <label className="bg-primary text-primary-foreground hover:bg-primary/90 inline-flex cursor-pointer items-center gap-2 rounded-lg px-3.5 py-2 text-xs font-semibold shadow-sm transition-colors">
                        {avatarUploading ? (
                          <Loader2 className="h-4 w-4 animate-spin" />
                        ) : (
                          <UploadCloud className="h-4 w-4" />
                        )}
                        <span>
                          {avatarUploading
                            ? t("profiles.actions.uploading")
                            : t("profiles.actions.upload_avatar")}
                        </span>
                        <input
                          type="file"
                          accept="image/png,image/jpeg,image/webp"
                          onChange={handleAvatarFileChange}
                          disabled={avatarUploading}
                          className="hidden"
                        />
                      </label>
                      {watchedValues.avatar_url && (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() =>
                            form.setValue("avatar_url", "", {
                              shouldDirty: true,
                            })
                          }
                          className="text-destructive hover:bg-destructive/10 text-xs"
                        >
                          {t("profiles.actions.remove")}
                        </Button>
                      )}
                    </div>
                  </div>
                </div>

                {/* Cover Uploader Dropzone */}
                <div className="border-border bg-muted/20 space-y-3 rounded-xl border border-dashed p-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-sm font-semibold">
                        {t("profiles.fields.cover")}
                      </div>
                      <p className="text-muted-foreground text-xs">
                        {t("profiles.fields.cover_hint")}
                      </p>
                    </div>
                    {watchedValues.cover_url && (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() =>
                          form.setValue("cover_url", "", { shouldDirty: true })
                        }
                        className="text-destructive hover:bg-destructive/10 text-xs"
                      >
                        {t("profiles.actions.remove_cover")}
                      </Button>
                    )}
                  </div>

                  <div className="border-border bg-muted/40 relative flex h-32 items-center justify-center overflow-hidden rounded-xl border">
                    {watchedValues.cover_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img
                        src={watchedValues.cover_url}
                        alt="Cover preview"
                        className="h-full w-full object-cover"
                      />
                    ) : (
                      <div className="space-y-1 text-center">
                        <ImageIcon className="text-muted-foreground mx-auto h-8 w-8" />
                        <span className="text-muted-foreground text-xs">
                          {t("profiles.fields.cover_hint")}
                        </span>
                      </div>
                    )}
                  </div>

                  <div className="flex justify-end pt-1">
                    <label className="border-input bg-background hover:bg-muted inline-flex cursor-pointer items-center gap-2 rounded-lg border px-3.5 py-2 text-xs font-semibold shadow-sm transition-colors">
                      {coverUploading ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <UploadCloud className="h-4 w-4" />
                      )}
                      <span>
                        {coverUploading
                          ? t("profiles.actions.uploading")
                          : t("profiles.actions.upload_cover")}
                      </span>
                      <input
                        type="file"
                        accept="image/png,image/jpeg,image/webp"
                        onChange={handleCoverFileChange}
                        disabled={coverUploading}
                        className="hidden"
                      />
                    </label>
                  </div>
                </div>
              </CardContent>
            </Card>
          )}

          {/* Tab 3: Contacts & Phone Inputs with Radio Selection */}
          {activeTab === "contacts" && (
            <Card className="border-border shadow-sm">
              <CardHeader className="flex flex-row items-center justify-between pb-4">
                <div>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Phone className="h-4 w-4 text-emerald-500" />
                    {t("profiles.sections.contacts_title")}
                  </CardTitle>
                  <CardDescription className="text-xs">
                    {t("profiles.sections.contacts_desc")}
                  </CardDescription>
                </div>
                <Button
                  type="button"
                  size="sm"
                  onClick={() =>
                    appendContact({
                      category: "phone",
                      type: "work",
                      label: "",
                      value: "",
                      is_primary: contactFields.length === 0,
                      display_order: contactFields.length,
                    })
                  }
                  className="gap-1.5 text-xs font-semibold"
                >
                  <Plus className="h-3.5 w-3.5" />
                  {t("profiles.actions.add_contact")}
                </Button>
              </CardHeader>
              <CardContent className="space-y-4 pt-0">
                {contactFields.length === 0 ? (
                  <div className="bg-muted/20 space-y-2 rounded-xl border border-dashed py-8 text-center">
                    <Phone className="text-muted-foreground mx-auto h-8 w-8" />
                    <p className="text-muted-foreground text-xs">
                      {t("profiles.empty_contacts")}
                    </p>
                  </div>
                ) : (
                  contactFields.map((field, idx) => {
                    const currentCategory =
                      watchedValues.contact_details?.[idx]?.category || "phone";
                    const isPhoneOrWhatsApp =
                      currentCategory === "phone" ||
                      currentCategory === "whatsapp";
                    const isPrimary = Boolean(
                      watchedValues.contact_details?.[idx]?.is_primary,
                    );

                    return (
                      <div
                        key={field.id}
                        className={cn(
                          "relative space-y-3 rounded-xl border p-4 transition-all",
                          isPrimary
                            ? "border-primary/50 bg-primary/5 shadow-xs"
                            : "border-border/60 bg-muted/20",
                        )}
                      >
                        <div className="border-border/40 flex items-center justify-between gap-2 border-b pb-2">
                          <div className="flex items-center gap-2">
                            <span className="text-muted-foreground text-xs font-bold">
                              #{idx + 1}
                            </span>
                            <span className="text-foreground text-xs font-semibold capitalize">
                              {t(
                                `profiles.contact.category_${currentCategory}`,
                              )}
                            </span>
                            {isPrimary && (
                              <Badge
                                variant="default"
                                className="gap-1 px-2 py-0 text-[10px]"
                              >
                                <Star className="h-3 w-3 fill-current" />
                                {t("profiles.contact.is_primary")}
                              </Badge>
                            )}
                          </div>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => removeContact(idx)}
                            className="text-muted-foreground hover:text-destructive h-7 w-7 p-0"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>

                        <div className="grid grid-cols-1 gap-3 sm:grid-cols-12">
                          <div className="sm:col-span-3">
                            <Label className="text-xs">
                              {t("profiles.contact.category")}
                            </Label>
                            <Controller
                              control={form.control}
                              name={`contact_details.${idx}.category`}
                              render={({ field }) => (
                                <Select
                                  onValueChange={(val) => {
                                    field.onChange(val);
                                  }}
                                  defaultValue={field.value}
                                >
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="phone">
                                      {t("profiles.contact.category_phone")}
                                    </SelectItem>
                                    <SelectItem value="email">
                                      {t("profiles.contact.category_email")}
                                    </SelectItem>
                                    <SelectItem value="whatsapp">
                                      {t("profiles.contact.category_whatsapp")}
                                    </SelectItem>
                                    <SelectItem value="url">
                                      {t("profiles.contact.category_url")}
                                    </SelectItem>
                                    <SelectItem value="other">
                                      {t("profiles.contact.category_other")}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              )}
                            />
                          </div>

                          <div className="sm:col-span-3">
                            <Label className="text-xs">
                              {t("profiles.contact.type")}
                            </Label>
                            <Controller
                              control={form.control}
                              name={`contact_details.${idx}.type`}
                              render={({ field }) => (
                                <Select
                                  onValueChange={field.onChange}
                                  defaultValue={field.value}
                                >
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="work">
                                      {t("profiles.contact.type_work")}
                                    </SelectItem>
                                    <SelectItem value="mobile">
                                      {t("profiles.contact.type_mobile")}
                                    </SelectItem>
                                    <SelectItem value="home">
                                      {t("profiles.contact.type_home")}
                                    </SelectItem>
                                    <SelectItem value="main">
                                      {t("profiles.contact.type_main")}
                                    </SelectItem>
                                    <SelectItem value="custom">
                                      {t("profiles.contact.type_custom")}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              )}
                            />
                          </div>

                          <div className="sm:col-span-6">
                            <Label className="text-xs">
                              {t("profiles.contact.value")}{" "}
                              <span className="text-destructive">*</span>
                            </Label>
                            {isPhoneOrWhatsApp ? (
                              <Controller
                                control={form.control}
                                name={`contact_details.${idx}.value`}
                                render={({ field: phoneField }) => (
                                  <div id={`field-contact-value-${idx}`}>
                                    <PhoneInput
                                      value={phoneField.value}
                                      onChange={(val) =>
                                        phoneField.onChange(val || "")
                                      }
                                      placeholder={
                                        currentCategory === "whatsapp"
                                          ? t(
                                              "profiles.contact.whatsapp_placeholder",
                                            )
                                          : t(
                                              "profiles.contact.phone_placeholder",
                                            )
                                      }
                                    />
                                  </div>
                                )}
                              />
                            ) : (
                              <Input
                                id={`field-contact-value-${idx}`}
                                placeholder={
                                  currentCategory === "email"
                                    ? t("profiles.contact.email_placeholder")
                                    : t("profiles.contact.url_placeholder")
                                }
                                {...form.register(
                                  `contact_details.${idx}.value`,
                                )}
                              />
                            )}
                          </div>
                        </div>

                        {/* Custom Label and Dynamic Radio Selection */}
                        <div className="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-12">
                          <div className="sm:col-span-7">
                            <Label className="text-xs">
                              {t("profiles.contact.label")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.contact.label_placeholder",
                              )}
                              {...form.register(`contact_details.${idx}.label`)}
                            />
                          </div>

                          <div className="flex items-end pb-1.5 sm:col-span-5">
                            <button
                              type="button"
                              onClick={() => handleSelectPrimaryContact(idx)}
                              className={cn(
                                "inline-flex w-full cursor-pointer items-center justify-between gap-2 rounded-lg border px-3 py-2 text-xs font-semibold transition-all",
                                isPrimary
                                  ? "bg-primary text-primary-foreground border-primary shadow-xs"
                                  : "bg-background border-input text-muted-foreground hover:text-foreground hover:bg-muted/50",
                              )}
                            >
                              <div className="flex items-center gap-2">
                                <div
                                  className={cn(
                                    "flex h-3.5 w-3.5 items-center justify-center rounded-full border",
                                    isPrimary
                                      ? "border-primary-foreground bg-primary-foreground"
                                      : "border-muted-foreground",
                                  )}
                                >
                                  {isPrimary && (
                                    <div className="bg-primary h-1.5 w-1.5 rounded-full" />
                                  )}
                                </div>
                                <span>{t("profiles.contact.is_primary")}</span>
                              </div>
                              {isPrimary && <Check className="h-3.5 w-3.5" />}
                            </button>
                          </div>
                        </div>
                      </div>
                    );
                  })
                )}
              </CardContent>
            </Card>
          )}

          {/* Tab 4: Online & Social Media Platforms */}
          {activeTab === "social" && (
            <Card className="border-border shadow-sm">
              <CardHeader className="flex flex-row items-center justify-between pb-4">
                <div>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Share2 className="h-4 w-4 text-blue-500" />
                    {t("profiles.sections.social_title")}
                  </CardTitle>
                  <CardDescription className="text-xs">
                    {t("profiles.sections.social_desc")}
                  </CardDescription>
                </div>
                <Button
                  type="button"
                  size="sm"
                  onClick={() =>
                    appendSocial({
                      platform_id: null,
                      platform: "linkedin",
                      label: "LinkedIn",
                      url: "",
                      icon_url: "",
                      display_order: socialFields.length,
                      is_active: true,
                    })
                  }
                  className="gap-1.5 text-xs font-semibold"
                >
                  <Plus className="h-3.5 w-3.5" />
                  {t("profiles.actions.add_social")}
                </Button>
              </CardHeader>
              <CardContent className="space-y-4 pt-0">
                {socialFields.length === 0 ? (
                  <div className="bg-muted/20 space-y-2 rounded-xl border border-dashed py-8 text-center">
                    <Share2 className="text-muted-foreground mx-auto h-8 w-8" />
                    <p className="text-muted-foreground text-xs">
                      {t("profiles.empty_social")}
                    </p>
                  </div>
                ) : (
                  socialFields.map((field, idx) => {
                    const currentSocial = watchedValues.social_links?.[idx];
                    return (
                      <div
                        key={field.id}
                        className="border-border/60 bg-muted/20 space-y-3 rounded-xl border p-4"
                      >
                        <div className="border-border/40 flex items-center justify-between gap-2 border-b pb-2">
                          <div className="flex items-center gap-2">
                            {currentSocial?.icon_url ? (
                              // eslint-disable-next-line @next/next/no-img-element
                              <img
                                src={currentSocial.icon_url}
                                alt="Icon"
                                className="h-4 w-4 object-contain"
                              />
                            ) : (
                              <Globe className="text-muted-foreground h-4 w-4" />
                            )}
                            <span className="text-xs font-bold capitalize">
                              {currentSocial?.label ||
                                currentSocial?.platform ||
                                t("profiles.tabs.social")}
                            </span>
                          </div>
                          <div className="flex items-center gap-1">
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              onClick={() => handleFetchLinkMetadata(idx)}
                              title={t("profiles.actions.fetch_icon")}
                              className="text-muted-foreground hover:text-foreground h-7 px-2 text-xs"
                            >
                              <Sparkles className="text-primary mr-1 h-3.5 w-3.5" />
                              <span>{t("profiles.actions.fetch_icon")}</span>
                            </Button>
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              onClick={() => removeSocial(idx)}
                              className="text-muted-foreground hover:text-destructive h-7 w-7 p-0"
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-3 sm:grid-cols-12">
                          <div className="sm:col-span-4">
                            <Label className="text-xs">
                              {t("profiles.social.platform")}
                            </Label>
                            <Select
                              defaultValue={
                                currentSocial?.platform || "linkedin"
                              }
                              onValueChange={(slug) =>
                                handleSelectPlatformPreset(idx, slug)
                              }
                            >
                              <SelectTrigger>
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {activePlatforms.map((p) => (
                                  <SelectItem key={p.slug} value={p.slug}>
                                    <div className="flex items-center gap-2">
                                      {p.icon_url ? (
                                        // eslint-disable-next-line @next/next/no-img-element
                                        <img
                                          src={p.icon_url}
                                          alt=""
                                          className="h-3.5 w-3.5 object-contain"
                                        />
                                      ) : null}
                                      <span>{p.name}</span>
                                    </div>
                                  </SelectItem>
                                ))}
                                <SelectItem value="custom">
                                  {t("profiles.social.custom")}
                                </SelectItem>
                              </SelectContent>
                            </Select>
                          </div>

                          <div className="sm:col-span-8">
                            <Label className="text-xs">
                              {t("profiles.social.url")}{" "}
                              <span className="text-destructive">*</span>
                            </Label>
                            <div className="relative">
                              <Input
                                id={`field-social-url-${idx}`}
                                placeholder={t(
                                  "profiles.social.url_placeholder",
                                )}
                                {...form.register(`social_links.${idx}.url`, {
                                  onChange: (e) => {
                                    debounceFetchMetadata(idx, e.target.value);
                                  },
                                })}
                              />
                              {scrapingIndex === idx && (
                                <div className="absolute top-1/2 right-3 -translate-y-1/2">
                                  <Loader2 className="text-primary h-4 w-4 animate-spin" />
                                </div>
                              )}
                            </div>
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-12">
                          <div className="sm:col-span-6">
                            <Label className="text-xs">
                              {t("profiles.social.label")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.social.label_placeholder",
                              )}
                              {...form.register(`social_links.${idx}.label`)}
                            />
                          </div>
                          <div className="flex items-end pb-1 sm:col-span-6">
                            <label className="flex cursor-pointer items-center gap-2 text-xs font-medium">
                              <input
                                type="checkbox"
                                {...form.register(
                                  `social_links.${idx}.is_active`,
                                )}
                                className="border-input text-primary focus:ring-primary rounded"
                              />
                              <span>{t("profiles.social.is_active")}</span>
                            </label>
                          </div>
                        </div>
                      </div>
                    );
                  })
                )}
              </CardContent>
            </Card>
          )}

          {/* Tab 5: Addresses & Locations with Radio Selection */}
          {activeTab === "addresses" && (
            <Card className="border-border shadow-sm">
              <CardHeader className="flex flex-row items-center justify-between pb-4">
                <div>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <MapPin className="h-4 w-4 text-rose-500" />
                    {t("profiles.sections.addresses_title")}
                  </CardTitle>
                  <CardDescription className="text-xs">
                    {t("profiles.sections.addresses_desc")}
                  </CardDescription>
                </div>
                <Button
                  type="button"
                  size="sm"
                  onClick={() =>
                    appendAddress({
                      type: "work",
                      label: "",
                      street: "",
                      extended: "",
                      city: "",
                      region: "",
                      postal_code: "",
                      country: "Turkey",
                      is_primary: addressFields.length === 0,
                      display_order: addressFields.length,
                    })
                  }
                  className="gap-1.5 text-xs font-semibold"
                >
                  <Plus className="h-3.5 w-3.5" />
                  {t("profiles.actions.add_address")}
                </Button>
              </CardHeader>
              <CardContent className="space-y-4 pt-0">
                {addressFields.length === 0 ? (
                  <div className="bg-muted/20 space-y-2 rounded-xl border border-dashed py-8 text-center">
                    <MapPin className="text-muted-foreground mx-auto h-8 w-8" />
                    <p className="text-muted-foreground text-xs">
                      {t("profiles.empty_addresses")}
                    </p>
                  </div>
                ) : (
                  addressFields.map((field, idx) => {
                    const isPrimary = Boolean(
                      watchedValues.addresses?.[idx]?.is_primary,
                    );

                    return (
                      <div
                        key={field.id}
                        className={cn(
                          "space-y-3 rounded-xl border p-4 transition-all",
                          isPrimary
                            ? "border-primary/50 bg-primary/5 shadow-xs"
                            : "border-border/60 bg-muted/20",
                        )}
                      >
                        <div className="border-border/40 flex items-center justify-between gap-2 border-b pb-2">
                          <div className="flex items-center gap-2">
                            <span className="text-muted-foreground text-xs font-bold">
                              #{idx + 1}
                            </span>
                            <span className="text-xs font-semibold capitalize">
                              {t(
                                `profiles.address.type_${watchedValues.addresses?.[idx]?.type || "work"}`,
                              )}
                            </span>
                            {isPrimary && (
                              <Badge
                                variant="default"
                                className="gap-1 px-2 py-0 text-[10px]"
                              >
                                <Star className="h-3 w-3 fill-current" />
                                {t("profiles.address.is_primary")}
                              </Badge>
                            )}
                          </div>
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => removeAddress(idx)}
                            className="text-muted-foreground hover:text-destructive h-7 w-7 p-0"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>

                        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.type")}
                            </Label>
                            <Controller
                              control={form.control}
                              name={`addresses.${idx}.type`}
                              render={({ field }) => (
                                <Select
                                  onValueChange={field.onChange}
                                  defaultValue={field.value}
                                >
                                  <SelectTrigger>
                                    <SelectValue />
                                  </SelectTrigger>
                                  <SelectContent>
                                    <SelectItem value="work">
                                      {t("profiles.address.type_work")}
                                    </SelectItem>
                                    <SelectItem value="home">
                                      {t("profiles.address.type_home")}
                                    </SelectItem>
                                    <SelectItem value="other">
                                      {t("profiles.address.type_other")}
                                    </SelectItem>
                                  </SelectContent>
                                </Select>
                              )}
                            />
                          </div>

                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.label")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.address.label_placeholder",
                              )}
                              {...form.register(`addresses.${idx}.label`)}
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.street")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.address.street_placeholder",
                              )}
                              {...form.register(`addresses.${idx}.street`)}
                            />
                          </div>
                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.extended")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.address.extended_placeholder",
                              )}
                              {...form.register(`addresses.${idx}.extended`)}
                            />
                          </div>
                        </div>

                        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.city")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.address.city_placeholder",
                              )}
                              {...form.register(`addresses.${idx}.city`)}
                            />
                          </div>
                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.region")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.address.region_placeholder",
                              )}
                              {...form.register(`addresses.${idx}.region`)}
                            />
                          </div>
                          <div>
                            <Label className="text-xs">
                              {t("profiles.address.country")}
                            </Label>
                            <Input
                              placeholder={t(
                                "profiles.address.country_placeholder",
                              )}
                              {...form.register(`addresses.${idx}.country`)}
                            />
                          </div>
                        </div>

                        <div className="flex justify-end pt-1">
                          <button
                            type="button"
                            onClick={() => handleSelectPrimaryAddress(idx)}
                            className={cn(
                              "inline-flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium transition-all",
                              isPrimary
                                ? "bg-primary text-primary-foreground border-primary"
                                : "bg-background border-input text-muted-foreground hover:text-foreground",
                            )}
                          >
                            <div
                              className={cn(
                                "flex h-3 w-3 items-center justify-center rounded-full border",
                                isPrimary
                                  ? "border-primary-foreground bg-primary-foreground"
                                  : "border-muted-foreground",
                              )}
                            >
                              {isPrimary && (
                                <div className="bg-primary h-1 w-1 rounded-full" />
                              )}
                            </div>
                            <span>{t("profiles.address.is_primary")}</span>
                          </button>
                        </div>
                      </div>
                    );
                  })
                )}
              </CardContent>
            </Card>
          )}

          {/* Tab 6: Settings */}
          {activeTab === "settings" && (
            <Card className="border-border shadow-sm">
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Sparkles className="h-4 w-4 text-amber-500" />
                  {t("profiles.sections.settings_title")}
                </CardTitle>
                <CardDescription className="text-xs">
                  {t("profiles.sections.settings_desc")}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Label className="text-xs font-semibold">
                    {t("profiles.fields.primary_locale")}
                  </Label>
                  <Controller
                    control={form.control}
                    name="primary_locale"
                    render={({ field }) => (
                      <Select
                        onValueChange={field.onChange}
                        defaultValue={field.value}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="tr">🇹🇷 Türkçe (TR)</SelectItem>
                          <SelectItem value="en">🇬🇧 English (EN)</SelectItem>
                        </SelectContent>
                      </Select>
                    )}
                  />
                </div>

                <div className="grid grid-cols-1 gap-4 pt-2 sm:grid-cols-2">
                  <div>
                    <Label className="text-xs font-semibold">
                      {t("profiles.fields.birthday")}
                    </Label>
                    <Controller
                      control={form.control}
                      name="birthday"
                      render={({ field }) => (
                        <DatePicker
                          value={field.value ?? undefined}
                          onChange={(val) => field.onChange(val || null)}
                          placeholder={t(
                            "profiles.fields.birthday_placeholder",
                          )}
                        />
                      )}
                    />
                  </div>

                  <div>
                    <Label className="text-xs font-semibold">
                      {t("profiles.fields.gender")}
                    </Label>
                    <Input
                      placeholder={t("profiles.fields.gender_placeholder")}
                      {...form.register("gender")}
                    />
                  </div>
                </div>

                <div className="pt-3">
                  <Controller
                    control={form.control}
                    name="is_primary"
                    render={({ field }) => (
                      <div className="border-border/60 bg-muted/20 flex items-center justify-between rounded-xl border p-3.5">
                        <div className="space-y-0.5">
                          <Label className="text-xs font-bold">
                            {t("profiles.fields.is_primary")}
                          </Label>
                          <p className="text-muted-foreground text-[11px]">
                            {t("profiles.fields.is_primary_description")}
                          </p>
                        </div>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </div>
                    )}
                  />
                </div>
              </CardContent>
            </Card>
          )}

          {/* Form Actions Footer */}
          <div className="border-border flex items-center justify-between border-t pt-4">
            <Button
              type="button"
              variant="outline"
              onClick={() => router.push(routes.app.profiles.root)}
            >
              {t("profiles.actions.cancel")}
            </Button>
            <Button
              type="button"
              onClick={handleFormSubmit}
              disabled={isLoading}
              className="min-w-[140px] gap-2"
            >
              {isLoading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Check className="h-4 w-4" />
              )}
              <span>
                {initialData
                  ? t("profiles.actions.save")
                  : t("profiles.actions.create_submit")}
              </span>
            </Button>
          </div>
        </div>

        {/* Right Column: In-Panel Live Digital Card Preview */}
        <div className="sticky top-6 space-y-4 lg:col-span-5">
          <div className="flex items-center justify-between">
            <h4 className="text-muted-foreground flex items-center gap-1.5 text-xs font-bold tracking-wider uppercase">
              <Sparkles className="text-primary h-3.5 w-3.5" />
              {t("profiles.preview.title")}
            </h4>
            <span className="text-muted-foreground text-[11px]">
              {t("profiles.preview.subtitle")}
            </span>
          </div>

          <ProfileCardPreview values={watchedValues} />
        </div>
      </div>
    </div>
  );
}
