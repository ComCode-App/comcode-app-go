"use client";

import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import { restrictToVerticalAxis } from "@dnd-kit/modifiers";
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ChevronDown,
  ChevronUp,
  ExternalLink,
  GripVertical,
  Plus,
  Trash2,
} from "lucide-react";
import { useMemo, useState } from "react";

import { Loading } from "@/components/common/loading";
import {
  EntityPage,
  EntitySectionCard,
  EntityUnavailable,
} from "@/components/entity";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { bankAccountsService } from "@/features/bank-accounts/services/bank-accounts.service";
import {
  companyBrandingKeys,
  companyBrandingService,
} from "@/features/company-branding/services/company-branding.service";
import { companyStructureService } from "@/features/company-structure/services/company-structure.service";
import { useDesignPremium } from "@/features/entitlements/components/premium-lock";
import { ProfileThemePicker } from "@/features/public-profile/components/theme-picker";
import type { ThemeId } from "@/features/public-profile/lib/themes";
import { socialPlatformsService } from "@/features/social-platforms/services/social-platforms.service";
import { ApiError } from "@/lib/api/errors";
import { cn } from "@/lib/utils";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

import { useBusinessPageAccess } from "../lib/access";
import {
  BLOCK_TYPES,
  BUTTON_ICONS,
  BUTTON_TARGETS,
  buttonIconElement,
  isUsernameShape,
  suggestUsername,
} from "../lib/blocks";
import {
  LOCALES,
  emptyState,
  fromPage,
  newBlock,
  toRequest,
  type BankAccountsConfig,
  type ButtonConfig,
  type EditorBlock,
  type EditorState,
  type HeaderConfig,
  type Locale,
  type SocialConfig,
} from "../lib/editor";
import {
  businessPageKeys,
  businessPageService,
  type BusinessBlockType,
  type BusinessButtonTarget,
  type BusinessPage,
  type PublicBusinessBlock,
  type PublicBusinessPage,
} from "../services/business-page.service";
import { BusinessPageView } from "./business-page-view";

const NONE = "__none__";

type Translate = ReturnType<typeof useLocale>["t"];

function errorMessage(err: unknown, t: Translate) {
  if (err instanceof ApiError) {
    if (err.status === 409) return t("business_page.errors.username_taken");
    if (err.status === 403 && err.code === "PLAN_LIMIT_EXCEEDED")
      return t("business_page.errors.plan");
    if (err.status === 403) return t("business_page.errors.forbidden");
  }
  return err instanceof Error && err.message
    ? err.message
    : t("common.error_generic");
}

/**
 * ComCode Business page editor (TEC-1327): blocks on the left (drag to reorder, switch on/off),
 * a live preview on the right, and the draft / published state. Company accounts only; members
 * see it read-only.
 */
export function BusinessPageEditorPage() {
  const { t } = useLocale();
  const access = useBusinessPageAccess();
  const pageQuery = useQuery({
    queryKey: businessPageKeys.page,
    queryFn: async () => {
      try {
        return await businessPageService.get();
      } catch (err) {
        // No page yet: the editor starts from a template.
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
      }
    },
    enabled: access.canRead,
  });

  return (
    <EntityPage
      title={t("business_page.title")}
      description={t("business_page.description")}
    >
      {!access.isCompany ? (
        <EntityUnavailable
          title={t("business_page.unavailable.personal_title")}
          description={t("business_page.unavailable.personal_description")}
        />
      ) : !access.canRead ? (
        <EntityUnavailable
          title={t("common.error_forbidden")}
          description={t("business_page.errors.forbidden")}
        />
      ) : pageQuery.isLoading ? (
        <Loading />
      ) : pageQuery.isError ? (
        <EntityUnavailable
          title={t("common.error")}
          description={errorMessage(pageQuery.error, t)}
        />
      ) : (
        <Editor
          key={pageQuery.data?.updated_at ?? "new"}
          page={pageQuery.data ?? null}
          canWrite={access.canWrite}
          companyName={access.accountName}
          companySlug={access.accountSlug}
        />
      )}
    </EntityPage>
  );
}

function Editor({
  page,
  canWrite,
  companyName,
  companySlug,
}: {
  page: BusinessPage | null;
  canWrite: boolean;
  companyName: string;
  companySlug: string;
}) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const { confirmDelete } = useDialogs();
  const [state, setState] = useState<EditorState>(() =>
    page
      ? fromPage(page)
      : emptyState(
          companyName,
          suggestUsername(companySlug || companyName) || "",
        ),
  );
  const [locale, setLocale] = useState<Locale>(state.primaryLocale);
  const [openKey, setOpenKey] = useState<string | null>(null);
  const savedJSON = useMemo(
    () => JSON.stringify(page ? toRequest(fromPage(page)) : null),
    [page],
  );
  const dirty = JSON.stringify(toRequest(state)) !== savedJSON;
  const allowPremium = useDesignPremium();

  // Data the blocks pick from and the preview shows.
  const library = useQuery({
    queryKey: companyBrandingKeys.media("all"),
    queryFn: () => companyBrandingService.listMedia(),
  });
  const branding = useQuery({
    queryKey: companyBrandingKeys.branding,
    queryFn: companyBrandingService.getBranding,
  });
  const offices = useQuery({
    queryKey: ["business-page", "offices"],
    queryFn: () => companyStructureService.listOffices(),
  });
  const banks = useQuery({
    queryKey: ["business-page", "bank-accounts"],
    queryFn: () => bankAccountsService.list(),
  });
  const platforms = useQuery({
    queryKey: ["business-page", "social-platforms"],
    queryFn: () => socialPlatformsService.listActive(),
  });

  const onSaved = (data: BusinessPage, message: string) => {
    queryClient.setQueryData(businessPageKeys.page, data);
    appToast.success(message);
  };
  const save = useMutation({
    mutationFn: () => businessPageService.save(toRequest(state)),
    onSuccess: (data) => onSaved(data, t("business_page.toast.saved")),
    onError: (err) => appToast.error(errorMessage(err, t)),
  });
  const publish = useMutation({
    mutationFn: async (published: boolean) => {
      // Unsaved changes go out first, so what is published is what the preview shows.
      if (dirty) await businessPageService.save(toRequest(state));
      return published
        ? businessPageService.publish()
        : businessPageService.unpublish();
    },
    onSuccess: (data, published) =>
      onSaved(
        data,
        published
          ? t("business_page.toast.published")
          : t("business_page.toast.unpublished"),
      ),
    onError: (err) => appToast.error(errorMessage(err, t)),
  });

  const busy = save.isPending || publish.isPending;
  const disabled = !canWrite || busy;
  const usernameValid = isUsernameShape(state.username.trim().toLowerCase());
  const published = page?.status === "published";

  const set = (patch: Partial<EditorState>) =>
    setState((s) => ({ ...s, ...patch }));
  const setBlock = (key: string, patch: Partial<EditorBlock>) =>
    setState((s) => ({
      ...s,
      blocks: s.blocks.map((b) => (b.key === key ? { ...b, ...patch } : b)),
    }));
  const addBlock = (type: BusinessBlockType) => {
    const b = newBlock(type);
    setState((s) => ({ ...s, blocks: [...s.blocks, b] }));
    setOpenKey(b.key);
  };
  const removeBlock = async (b: EditorBlock) => {
    const ok = await confirmDelete({
      title: t("business_page.blocks.delete_title"),
      description: t("business_page.blocks.delete_description", {
        name: t(`business_page.block.${b.type}`),
      }),
    });
    if (ok)
      setState((s) => ({
        ...s,
        blocks: s.blocks.filter((x) => x.key !== b.key),
      }));
  };

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );
  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return;
    setState((s) => {
      const from = s.blocks.findIndex((b) => b.key === active.id);
      const to = s.blocks.findIndex((b) => b.key === over.id);
      return { ...s, blocks: arrayMove(s.blocks, from, to) };
    });
  };

  const images = (library.data?.items ?? []).filter(
    (m) => m.kind !== "document",
  );
  const preview = useMemo<PublicBusinessPage>(() => {
    const media = new Map(images.map((m) => [m.uuid, m.url]));
    const icons = new Map(
      (platforms.data ?? []).map((p) => [p.slug, p.icon_url]),
    );
    const b = branding.data;
    let logo = b?.logo?.url ?? "";
    let cover = b?.cover?.url ?? "";
    const blocks: PublicBusinessBlock[] = [];
    for (const blk of state.blocks) {
      if (!blk.enabled) continue;
      const out: PublicBusinessBlock = {
        type: blk.type,
        translations: blk.translations as PublicBusinessBlock["translations"],
      };
      if (blk.type === "header") {
        const c = blk.config as HeaderConfig;
        if (c.logo_media_uuid) logo = media.get(c.logo_media_uuid) ?? logo;
        if (c.cover_media_uuid) cover = media.get(c.cover_media_uuid) ?? cover;
      } else if (blk.type === "button") {
        const c = blk.config as ButtonConfig;
        out.button = {
          target: c.target as BusinessButtonTarget,
          value: c.value,
          icon: c.icon,
        };
      } else if (blk.type === "social") {
        out.social = (blk.config as SocialConfig).links
          .filter((l) => l.platform && l.url)
          .map((l) => ({ ...l, icon_url: icons.get(l.platform) ?? "" }));
      } else if (blk.type === "offices") {
        out.offices = (offices.data ?? []).map((o) => ({
          name: o.name,
          city: o.city ?? "",
          district: o.district ?? "",
          address: o.address ?? "",
          phone: o.phone ?? "",
        }));
      } else if (blk.type === "team") {
        out.team = [];
      } else if (blk.type === "bank_accounts") {
        const picked = (blk.config as BankAccountsConfig).bank_account_uuids;
        out.bank_accounts = (banks.data ?? [])
          .filter((a) => a.is_active && picked.includes(a.uuid))
          .map((a) => ({
            bank_name: a.bank_name ?? "",
            bank_icon_url: a.bank_icon_url ?? "",
            bank_brand_color: a.bank_brand_color ?? "",
            account_holder: a.account_holder,
            iban: a.iban,
            currency: a.currency,
            title: a.title,
          }));
      }
      blocks.push(out);
    }
    return {
      username: state.username,
      page_uuid: page?.uuid ?? "",
      company_name: companyName,
      primary_locale: state.primaryLocale,
      translations: state.translations as PublicBusinessPage["translations"],
      theme: state.theme || b?.theme || "",
      primary_color: b?.primary_color ?? "",
      logo_url: logo,
      cover_url: cover,
      blocks,
    };
  }, [
    state,
    images,
    platforms.data,
    branding.data,
    offices.data,
    banks.data,
    page,
    companyName,
  ]);

  const pageText = state.translations[locale] ?? { name: "", tagline: "" };
  const publicHref = page?.username ? `/${page.username}` : null;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_400px]">
      <div className="min-w-0 space-y-6">
        <EntitySectionCard title={t("business_page.status.title")}>
          <div className="flex flex-wrap items-center gap-3">
            <Badge variant={published ? "success" : "outline"}>
              {published
                ? t("business_page.status.published")
                : t("business_page.status.draft")}
            </Badge>
            {publicHref && published ? (
              <a
                href={publicHref}
                target="_blank"
                rel="noopener noreferrer"
                className="text-primary inline-flex items-center gap-1 text-sm hover:underline"
              >
                {publicHref}
                <ExternalLink className="size-3.5" />
              </a>
            ) : null}
            {dirty ? (
              <span className="text-muted-foreground text-xs">
                {t("business_page.status.unsaved")}
              </span>
            ) : null}
            {canWrite ? (
              <div className="ms-auto flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="outline"
                  disabled={disabled || !usernameValid || (!dirty && !!page)}
                  onClick={() => save.mutate()}
                >
                  {t("business_page.actions.save")}
                </Button>
                {published ? (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={disabled}
                    onClick={() => publish.mutate(false)}
                  >
                    {t("business_page.actions.unpublish")}
                  </Button>
                ) : (
                  <Button
                    type="button"
                    disabled={disabled || !usernameValid}
                    onClick={() => publish.mutate(true)}
                  >
                    {t("business_page.actions.publish")}
                  </Button>
                )}
              </div>
            ) : (
              <p className="text-muted-foreground ms-auto text-xs">
                {t("business_page.read_only")}
              </p>
            )}
          </div>
        </EntitySectionCard>

        <EntitySectionCard title={t("business_page.settings.title")}>
          <div className="space-y-5">
            <div className="grid gap-1.5">
              <Label htmlFor="bp-username">
                {t("business_page.settings.username")}
              </Label>
              <div className="flex items-center gap-1">
                <span className="text-muted-foreground text-sm">/</span>
                <Input
                  id="bp-username"
                  value={state.username}
                  onChange={(e) => set({ username: e.target.value })}
                  disabled={disabled}
                  maxLength={40}
                  className="max-w-xs font-mono"
                  aria-invalid={!usernameValid}
                />
              </div>
              <p
                className={cn(
                  "text-xs",
                  usernameValid ? "text-muted-foreground" : "text-destructive",
                )}
              >
                {usernameValid
                  ? t("business_page.settings.username_hint")
                  : t("business_page.settings.username_invalid")}
              </p>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-1.5">
                <Label htmlFor="bp-locale">
                  {t("business_page.settings.primary_locale")}
                </Label>
                <Select
                  value={state.primaryLocale}
                  onValueChange={(v) => set({ primaryLocale: v as Locale })}
                  disabled={disabled}
                >
                  <SelectTrigger id="bp-locale" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {LOCALES.map((l) => (
                      <SelectItem key={l} value={l}>
                        {t(`business_page.locale.${l}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-1.5">
                <Label>{t("business_page.settings.editing_locale")}</Label>
                <Tabs
                  value={locale}
                  onValueChange={(v) => setLocale(v as Locale)}
                >
                  <TabsList>
                    {LOCALES.map((l) => (
                      <TabsTrigger key={l} value={l}>
                        {t(`business_page.locale.${l}`)}
                      </TabsTrigger>
                    ))}
                  </TabsList>
                </Tabs>
              </div>
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-1.5">
                <Label htmlFor="bp-name">
                  {t("business_page.settings.name")}
                </Label>
                <Input
                  id="bp-name"
                  value={pageText.name}
                  maxLength={120}
                  disabled={disabled}
                  placeholder={companyName}
                  onChange={(e) =>
                    set({
                      translations: {
                        ...state.translations,
                        [locale]: { ...pageText, name: e.target.value },
                      },
                    })
                  }
                />
              </div>
              <div className="grid gap-1.5">
                <Label htmlFor="bp-tagline">
                  {t("business_page.settings.tagline")}
                </Label>
                <Input
                  id="bp-tagline"
                  value={pageText.tagline}
                  maxLength={200}
                  disabled={disabled}
                  onChange={(e) =>
                    set({
                      translations: {
                        ...state.translations,
                        [locale]: { ...pageText, tagline: e.target.value },
                      },
                    })
                  }
                />
              </div>
            </div>

            <ProfileThemePicker
              theme={state.theme as ThemeId | ""}
              inheritedTheme={(branding.data?.theme ?? "") as ThemeId | ""}
              disabled={disabled}
              noneLabel={t("business_page.settings.theme_company")}
              hint={t("business_page.settings.theme_hint")}
              premiumAllowed={allowPremium}
              onThemeChange={(id) => set({ theme: id })}
            />
          </div>
        </EntitySectionCard>

        <EntitySectionCard title={t("business_page.blocks.title")}>
          <div className="mb-3 flex items-center justify-between gap-2">
            <p className="text-muted-foreground text-xs">
              {t("business_page.blocks.hint")}
            </p>
            {canWrite ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" size="sm" disabled={disabled}>
                    <Plus className="size-4" />
                    {t("business_page.blocks.add")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {BLOCK_TYPES.map((type) => (
                    <DropdownMenuItem
                      key={type}
                      onSelect={() => addBlock(type)}
                    >
                      {t(`business_page.block.${type}`)}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            ) : null}
          </div>
          {state.blocks.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              {t("business_page.blocks.empty")}
            </p>
          ) : (
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              modifiers={[restrictToVerticalAxis]}
              onDragEnd={onDragEnd}
            >
              <SortableContext
                items={state.blocks.map((b) => b.key)}
                strategy={verticalListSortingStrategy}
              >
                <ul className="space-y-2">
                  {state.blocks.map((b) => (
                    <SortableBlock
                      key={b.key}
                      block={b}
                      locale={locale}
                      open={openKey === b.key}
                      disabled={disabled}
                      onToggleOpen={() =>
                        setOpenKey(openKey === b.key ? null : b.key)
                      }
                      onChange={(patch) => setBlock(b.key, patch)}
                      onRemove={() => void removeBlock(b)}
                      images={images.map((m) => ({
                        uuid: m.uuid,
                        name: m.name,
                      }))}
                      platforms={(platforms.data ?? []).map((p) => ({
                        slug: p.slug,
                        name: p.name,
                      }))}
                      banks={(banks.data ?? [])
                        .filter((a) => a.is_active)
                        .map((a) => ({
                          uuid: a.uuid,
                          label: [a.title || a.bank_name, a.formatted_iban]
                            .filter(Boolean)
                            .join(" · "),
                        }))}
                    />
                  ))}
                </ul>
              </SortableContext>
            </DndContext>
          )}
        </EntitySectionCard>
      </div>

      <aside className="min-w-0 lg:sticky lg:top-4 lg:self-start">
        <p className="text-muted-foreground mb-2 text-xs font-medium tracking-wide uppercase">
          {t("business_page.preview")}
        </p>
        <div className="h-[720px] overflow-y-auto rounded-3xl border shadow-sm">
          <BusinessPageView
            page={preview}
            locale={locale}
            locales={LOCALES.filter(
              (l) => l === state.primaryLocale || state.translations[l]?.name,
            )}
            onLocaleChange={(l) => setLocale(l as Locale)}
            preview
          />
        </div>
      </aside>
    </div>
  );
}

type Option = { uuid: string; name: string };

function SortableBlock({
  block,
  locale,
  open,
  disabled,
  onToggleOpen,
  onChange,
  onRemove,
  images,
  platforms,
  banks,
}: {
  block: EditorBlock;
  locale: Locale;
  open: boolean;
  disabled: boolean;
  onToggleOpen: () => void;
  onChange: (patch: Partial<EditorBlock>) => void;
  onRemove: () => void;
  images: Option[];
  platforms: { slug: string; name: string }[];
  banks: { uuid: string; label: string }[];
}) {
  const { t } = useLocale();
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: block.key, disabled });
  const texts = block.translations[locale] ?? { title: "", subtitle: "" };
  const setText = (patch: Partial<typeof texts>) =>
    onChange({
      translations: { ...block.translations, [locale]: { ...texts, ...patch } },
    });
  const setConfig = (patch: Record<string, unknown>) =>
    onChange({ config: { ...block.config, ...patch } });

  const button = block.config as ButtonConfig;
  const summary =
    block.type === "button"
      ? texts.title || t(`public_profile.business.target.${button.target}`)
      : texts.title;
  const icon =
    block.type === "button"
      ? buttonIconElement(
          button.target,
          button.icon,
          "text-muted-foreground size-4 shrink-0",
        )
      : null;

  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        "bg-card rounded-xl border",
        isDragging && "relative z-10 shadow-lg",
        !block.enabled && "opacity-70",
      )}
    >
      <div className="flex items-center gap-2 p-2">
        <button
          type="button"
          className="text-muted-foreground hover:text-foreground cursor-grab rounded p-1 disabled:cursor-not-allowed"
          aria-label={t("business_page.blocks.drag")}
          disabled={disabled}
          {...attributes}
          {...listeners}
        >
          <GripVertical className="size-4" />
        </button>
        <button
          type="button"
          onClick={onToggleOpen}
          className="flex min-w-0 flex-1 items-center gap-2 text-left"
          aria-expanded={open}
        >
          {icon}
          <span className="text-sm font-medium">
            {t(`business_page.block.${block.type}`)}
          </span>
          {summary ? (
            <span className="text-muted-foreground truncate text-sm">
              · {summary}
            </span>
          ) : null}
          {open ? (
            <ChevronUp className="ms-auto size-4 shrink-0" />
          ) : (
            <ChevronDown className="ms-auto size-4 shrink-0" />
          )}
        </button>
        <Switch
          checked={block.enabled}
          onCheckedChange={(v) => onChange({ enabled: v })}
          disabled={disabled}
          aria-label={t("business_page.blocks.enabled")}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-8"
          onClick={onRemove}
          disabled={disabled}
          aria-label={t("business_page.blocks.delete")}
        >
          <Trash2 className="size-4" />
        </Button>
      </div>

      {open ? (
        <div className="space-y-4 border-t p-4">
          {block.type === "header" ? (
            <div className="grid gap-4 sm:grid-cols-2">
              {(["logo_media_uuid", "cover_media_uuid"] as const).map(
                (field) => (
                  <div key={field} className="grid gap-1.5">
                    <Label htmlFor={`${block.key}-${field}`}>
                      {t(`business_page.header.${field}`)}
                    </Label>
                    <Select
                      value={
                        ((block.config as HeaderConfig)[field] as string) ||
                        NONE
                      }
                      onValueChange={(v) =>
                        setConfig({ [field]: v === NONE ? null : v })
                      }
                      disabled={disabled}
                    >
                      <SelectTrigger
                        id={`${block.key}-${field}`}
                        className="w-full"
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={NONE}>
                          {t("business_page.header.branding")}
                        </SelectItem>
                        {images.map((m) => (
                          <SelectItem key={m.uuid} value={m.uuid}>
                            {m.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                ),
              )}
              <p className="text-muted-foreground text-xs sm:col-span-2">
                {t("business_page.header.hint")}
              </p>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-1.5">
                <Label htmlFor={`${block.key}-title`}>
                  {block.type === "button"
                    ? t("business_page.button.title")
                    : t("business_page.blocks.section_title")}
                </Label>
                <Input
                  id={`${block.key}-title`}
                  value={texts.title}
                  maxLength={120}
                  disabled={disabled}
                  onChange={(e) => setText({ title: e.target.value })}
                />
              </div>
              {block.type === "button" ? (
                <div className="grid gap-1.5">
                  <Label htmlFor={`${block.key}-subtitle`}>
                    {t("business_page.button.subtitle")}
                  </Label>
                  <Input
                    id={`${block.key}-subtitle`}
                    value={texts.subtitle}
                    maxLength={200}
                    disabled={disabled}
                    onChange={(e) => setText({ subtitle: e.target.value })}
                  />
                </div>
              ) : null}
            </div>
          )}

          {block.type === "button" ? (
            <div className="grid gap-4 sm:grid-cols-3">
              <div className="grid gap-1.5">
                <Label htmlFor={`${block.key}-target`}>
                  {t("business_page.button.target")}
                </Label>
                <Select
                  value={button.target}
                  onValueChange={(v) => setConfig({ target: v })}
                  disabled={disabled}
                >
                  <SelectTrigger id={`${block.key}-target`} className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {BUTTON_TARGETS.map((target) => (
                      <SelectItem key={target} value={target}>
                        {t(`public_profile.business.target.${target}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {button.target !== "lead_form" ? (
                <div className="grid gap-1.5">
                  <Label htmlFor={`${block.key}-value`}>
                    {t(`business_page.button.value.${button.target}`)}
                  </Label>
                  <Input
                    id={`${block.key}-value`}
                    value={button.value}
                    maxLength={2048}
                    disabled={disabled}
                    placeholder={t(
                      `business_page.button.placeholder.${button.target}`,
                    )}
                    onChange={(e) => setConfig({ value: e.target.value })}
                  />
                </div>
              ) : (
                <p className="text-muted-foreground self-end text-xs">
                  {t("business_page.button.lead_form_hint")}
                </p>
              )}
              <div className="grid gap-1.5">
                <Label htmlFor={`${block.key}-icon`}>
                  {t("business_page.button.icon")}
                </Label>
                <Select
                  value={button.icon || NONE}
                  onValueChange={(v) =>
                    setConfig({ icon: v === NONE ? "" : v })
                  }
                  disabled={disabled}
                >
                  <SelectTrigger id={`${block.key}-icon`} className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>
                      {t("business_page.button.icon_default")}
                    </SelectItem>
                    {Object.entries(BUTTON_ICONS).map(([name, I]) => (
                      <SelectItem key={name} value={name}>
                        <span className="flex items-center gap-2">
                          <I className="size-4" />
                          {t(`business_page.icon.${name}`)}
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          ) : null}

          {block.type === "social" ? (
            <SocialLinksEditor
              id={block.key}
              links={(block.config as SocialConfig).links}
              platforms={platforms}
              disabled={disabled}
              onChange={(links) => setConfig({ links })}
            />
          ) : null}

          {block.type === "bank_accounts" ? (
            banks.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                {t("business_page.bank_accounts.none")}
              </p>
            ) : (
              <ul className="space-y-2">
                {banks.map((a) => {
                  const picked = (block.config as BankAccountsConfig)
                    .bank_account_uuids;
                  const id = `${block.key}-${a.uuid}`;
                  return (
                    <li key={a.uuid} className="flex items-center gap-2">
                      <Checkbox
                        id={id}
                        checked={picked.includes(a.uuid)}
                        disabled={disabled}
                        onCheckedChange={(v) =>
                          setConfig({
                            bank_account_uuids: v
                              ? [...picked, a.uuid]
                              : picked.filter((x) => x !== a.uuid),
                          })
                        }
                      />
                      <Label htmlFor={id} className="font-normal">
                        {a.label}
                      </Label>
                    </li>
                  );
                })}
              </ul>
            )
          ) : null}

          {block.type === "offices" || block.type === "team" ? (
            <p className="text-muted-foreground text-xs">
              {t(`business_page.${block.type}.hint`)}
            </p>
          ) : null}
        </div>
      ) : null}
    </li>
  );
}

function SocialLinksEditor({
  id,
  links,
  platforms,
  disabled,
  onChange,
}: {
  id: string;
  links: SocialConfig["links"];
  platforms: { slug: string; name: string }[];
  disabled: boolean;
  onChange: (links: SocialConfig["links"]) => void;
}) {
  const { t } = useLocale();
  const update = (i: number, patch: Partial<SocialConfig["links"][number]>) =>
    onChange(links.map((l, j) => (j === i ? { ...l, ...patch } : l)));
  return (
    <div className="space-y-2">
      {links.map((l, i) => (
        <div key={i} className="flex flex-wrap items-center gap-2">
          <Select
            value={l.platform || NONE}
            onValueChange={(v) => update(i, { platform: v === NONE ? "" : v })}
            disabled={disabled}
          >
            <SelectTrigger
              className="w-40"
              aria-label={t("business_page.social.platform")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>
                {t("business_page.social.pick")}
              </SelectItem>
              {platforms.map((p) => (
                <SelectItem key={p.slug} value={p.slug}>
                  {p.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Input
            id={`${id}-social-${i}`}
            value={l.url}
            placeholder="https://"
            maxLength={2048}
            disabled={disabled}
            aria-label={t("business_page.social.url")}
            className="min-w-0 flex-1"
            onChange={(e) => update(i, { url: e.target.value })}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-8"
            disabled={disabled}
            aria-label={t("business_page.social.remove")}
            onClick={() => onChange(links.filter((_, j) => j !== i))}
          >
            <Trash2 className="size-4" />
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled || links.length >= 20}
        onClick={() => onChange([...links, { platform: "", url: "" }])}
      >
        <Plus className="size-4" />
        {t("business_page.social.add")}
      </Button>
    </div>
  );
}
