"use client";

import { useState } from "react";
import { useForm, Controller } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { Globe, Loader2, Sparkles, UploadCloud } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  type SocialPlatformSummary,
  type SocialPlatformDetail,
} from "../services/social-platforms.service";
import {
  socialPlatformFormSchema,
  type SocialPlatformFormValues,
} from "../schemas/social-platform-form";
import { useSocialPlatformMutations } from "../hooks/use-social-platforms";
import { profilesService } from "@/features/profiles/services/profiles.service";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

interface SocialPlatformDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  platform?: SocialPlatformSummary | SocialPlatformDetail | null;
}

const CATEGORY_KEYS = [
  "social",
  "professional",
  "developer",
  "messaging",
  "media",
  "community",
  "portfolio",
  "publishing",
  "general",
] as const;

export function SocialPlatformDialog({
  open,
  onOpenChange,
  platform,
}: SocialPlatformDialogProps) {
  const { t } = useLocale();
  const isEditing = Boolean(platform);
  const {
    createMutation,
    updateMutation,
    uploadIconMutation,
    fetchIconMutation,
  } = useSocialPlatformMutations();
  const [iconFile, setIconFile] = useState<File | null>(null);
  const [customIconUrl, setCustomIconUrl] = useState<string | null>(null);
  const [imgError, setImgError] = useState(false);
  const [isFetchingOG, setIsFetchingOG] = useState(false);

  const iconPreview = customIconUrl ?? platform?.icon_url ?? null;

  const form = useForm<SocialPlatformFormValues>({
    resolver: zodResolver(socialPlatformFormSchema),
    values: {
      name: platform?.name || "",
      slug: platform?.slug || "",
      category: platform?.category || "social",
      url_pattern: platform?.url_pattern || "https://example.com/{value}",
      placeholder: platform?.placeholder || "username",
      description: platform?.description || "",
      display_order: platform?.display_order || 0,
      is_active: platform?.is_active ?? true,
    },
  });

  const handleIconChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setIconFile(file);
      const url = URL.createObjectURL(file);
      setCustomIconUrl(url);
      setImgError(false);
    }
  };

  const handleFetchOGIcon = async () => {
    if (isEditing && platform?.uuid) {
      try {
        setIsFetchingOG(true);
        const res = await fetchIconMutation.mutateAsync(platform.uuid);
        if (res.icon_url) {
          setCustomIconUrl(res.icon_url);
          setImgError(false);
        }
      } finally {
        setIsFetchingOG(false);
      }
    } else {
      const pattern = form.getValues("url_pattern");
      const target = pattern
        ? pattern.replace("{value}", "").trim()
        : `https://${form.getValues("slug") || "example"}.com`;

      try {
        setIsFetchingOG(true);
        const meta = await profilesService.fetchLinkMetadata(target);
        if (meta.icon_url) {
          setCustomIconUrl(meta.icon_url);
          setImgError(false);
          appToast.success(t("profiles.toasts.metadata_fetched"));
        } else {
          appToast.error(t("profiles.toasts.metadata_failed"));
        }
      } catch {
        appToast.error(t("profiles.toasts.metadata_failed"));
      } finally {
        setIsFetchingOG(false);
      }
    }
  };

  const onSubmit = async (values: SocialPlatformFormValues) => {
    try {
      let savedUUID = platform?.uuid;

      if (isEditing && platform) {
        await updateMutation.mutateAsync({
          uuid: platform.uuid,
          body: {
            name: values.name,
            slug: values.slug,
            category: values.category,
            url_pattern: values.url_pattern,
            placeholder: values.placeholder || "",
            description: values.description || "",
            display_order: values.display_order,
            is_active: values.is_active,
          },
        });
      } else {
        const created = await createMutation.mutateAsync({
          name: values.name,
          slug: values.slug,
          category: values.category,
          url_pattern: values.url_pattern,
          placeholder: values.placeholder || "",
          description: values.description || "",
          display_order: values.display_order,
          is_active: values.is_active,
        });
        savedUUID = created.uuid;
      }

      if (iconFile && savedUUID) {
        await uploadIconMutation.mutateAsync({
          uuid: savedUUID,
          file: iconFile,
        });
      }

      onOpenChange(false);
    } catch {
      // Handled by hook toasts
    }
  };

  const isSubmitting =
    createMutation.isPending ||
    updateMutation.isPending ||
    uploadIconMutation.isPending ||
    isFetchingOG;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-md overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEditing
              ? t("social_platforms.dialog.edit_title", {
                  name: platform?.name || "",
                })
              : t("social_platforms.dialog.create_title")}
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
          {/* Icon Picker */}
          <div className="space-y-2">
            <Label className="text-xs">
              {t("social_platforms.dialog.icon_label")}
            </Label>
            <div className="flex items-center gap-4">
              <div className="border-border bg-muted/30 flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-xl border">
                {iconPreview && !imgError ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={iconPreview}
                    alt="Icon Preview"
                    onError={() => setImgError(true)}
                    className="h-10 w-10 object-contain"
                  />
                ) : (
                  <Globe className="text-muted-foreground h-6 w-6" />
                )}
              </div>
              <div className="flex-1 space-y-1.5">
                <div className="flex flex-wrap items-center gap-2">
                  <label className="border-input hover:bg-muted inline-flex cursor-pointer items-center gap-2 rounded-md border px-3 py-1.5 text-xs font-medium transition-colors">
                    <UploadCloud className="text-muted-foreground h-3.5 w-3.5" />
                    <span>{t("social_platforms.actions.upload_file")}</span>
                    <input
                      type="file"
                      accept="image/svg+xml,image/png,image/jpeg,image/webp"
                      onChange={handleIconChange}
                      className="hidden"
                    />
                  </label>

                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleFetchOGIcon}
                    disabled={isFetchingOG}
                    className="h-8 gap-1.5 text-xs font-medium"
                  >
                    {isFetchingOG ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Sparkles className="text-primary h-3.5 w-3.5" />
                    )}
                    <span>{t("social_platforms.actions.fetch_icon_og")}</span>
                  </Button>
                </div>
                <p className="text-muted-foreground text-[11px]">
                  {t("social_platforms.dialog.icon_hint")}
                </p>
              </div>
            </div>
          </div>

          <div>
            <Label className="text-xs">{t("social_platforms.name")}</Label>
            <Input placeholder="e.g. LinkedIn" {...form.register("name")} />
            {form.formState.errors.name && (
              <p className="text-destructive mt-1 text-xs">
                {form.formState.errors.name.message}
              </p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs">{t("social_platforms.slug")}</Label>
              <Input placeholder="e.g. linkedin" {...form.register("slug")} />
            </div>

            <div>
              <Label className="text-xs">
                {t("social_platforms.category")}
              </Label>
              <Controller
                control={form.control}
                name="category"
                render={({ field }) => (
                  <Select
                    onValueChange={field.onChange}
                    defaultValue={field.value}
                  >
                    <SelectTrigger>
                      <SelectValue
                        placeholder={t("social_platforms.category")}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {CATEGORY_KEYS.map((catKey) => (
                        <SelectItem key={catKey} value={catKey}>
                          {t(`social_platforms.categories.${catKey}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
            </div>
          </div>

          <div>
            <Label className="text-xs">
              {t("social_platforms.url_pattern")}
            </Label>
            <Input
              placeholder="https://linkedin.com/in/{value}"
              {...form.register("url_pattern")}
            />
            <p className="text-muted-foreground mt-1 text-[11px]">
              {t("social_platforms.dialog.url_pattern_hint")}
            </p>
            {form.formState.errors.url_pattern && (
              <p className="text-destructive mt-1 text-xs">
                {form.formState.errors.url_pattern.message}
              </p>
            )}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs">
                {t("social_platforms.placeholder")}
              </Label>
              <Input
                placeholder="e.g. username"
                {...form.register("placeholder")}
              />
            </div>

            <div>
              <Label className="text-xs">
                {t("social_platforms.display_order")}
              </Label>
              <Input
                type="number"
                {...form.register("display_order", { valueAsNumber: true })}
              />
            </div>
          </div>

          <div>
            <Label className="text-xs">
              {t("social_platforms.description")}
            </Label>
            <Textarea
              placeholder="Platform description..."
              rows={2}
              {...form.register("description")}
            />
          </div>

          <Controller
            control={form.control}
            name="is_active"
            render={({ field }) => (
              <div className="flex items-center justify-between rounded-lg border p-3">
                <div className="space-y-0.5">
                  <Label className="text-xs font-semibold">
                    {t("social_platforms.dialog.is_active_title")}
                  </Label>
                  <p className="text-muted-foreground text-[11px]">
                    {t("social_platforms.dialog.is_active_hint")}
                  </p>
                </div>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </div>
            )}
          />

          <DialogFooter className="pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <Loader2 className="mr-1 h-4 w-4 animate-spin" />
              ) : null}
              <span>{t("common.save")}</span>
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
