"use client";

import { useState, useRef, useCallback } from "react";
import {
  FileCode2,
  UploadCloud,
  CheckCircle2,
  Sparkles,
  User,
  Briefcase,
  Phone,
  Mail,
  MapPin,
  Share2,
  Calendar,
  FileText,
  Loader2,
  ArrowRight,
  RefreshCw,
  Sliders,
} from "lucide-react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Switch } from "@/components/ui/switch";
import {
  type ParsedVCardProfile,
  profilesService,
} from "../services/profiles.service";
import { appToast } from "@/providers/toast-provider";
import { useLocale } from "@/providers/locale-provider";
import { cn } from "@/lib/utils";
import { isApiError } from "@/lib/api";

export interface VCardApplyOptions {
  includeIdentity: boolean;
  includeWork: boolean;
  includeContacts: boolean;
  includeAddresses: boolean;
  includeSocials: boolean;
  includeBio: boolean;
  includeAvatar: boolean;
  mergeMode: "merge" | "replace";
}

interface VCardImportDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onApply: (data: ParsedVCardProfile, options: VCardApplyOptions) => void;
}

export function VCardImportDialog({
  open,
  onOpenChange,
  onApply,
}: VCardImportDialogProps) {
  const { t } = useLocale();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [fileName, setFileName] = useState<string | null>(null);
  const [parsedData, setParsedData] = useState<ParsedVCardProfile | null>(null);

  // Field selection toggles
  const [includeIdentity, setIncludeIdentity] = useState(true);
  const [includeWork, setIncludeWork] = useState(true);
  const [includeContacts, setIncludeContacts] = useState(true);
  const [includeAddresses, setIncludeAddresses] = useState(true);
  const [includeSocials, setIncludeSocials] = useState(true);
  const [includeBio] = useState(true);
  const [includeAvatar, setIncludeAvatar] = useState(true);
  const [mergeMode, setMergeMode] = useState<"merge" | "replace">("replace");

  const resetState = useCallback(() => {
    setFileName(null);
    setParsedData(null);
    setIsLoading(false);
    setIsDragging(false);
  }, []);

  const handleFile = async (file: File) => {
    if (
      !file.name.toLowerCase().endsWith(".vcf") &&
      !file.name.toLowerCase().endsWith(".vcard")
    ) {
      appToast.error(t("profiles.vcard.invalid_extension"));
      return;
    }

    try {
      setIsLoading(true);
      setFileName(file.name);
      const data = await profilesService.parseVCard(file);
      setParsedData(data);
      appToast.success(t("profiles.vcard.parse_success"));
    } catch (err: unknown) {
      // API errors already get the global toast (TEC-1117).
      if (!isApiError(err)) {
        appToast.error(
          err instanceof Error ? err.message : t("profiles.vcard.parse_failed"),
        );
      }
      resetState();
    } finally {
      setIsLoading(false);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const file = e.dataTransfer.files?.[0];
    if (file) {
      handleFile(file);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      handleFile(file);
    }
  };

  const handleConfirm = () => {
    if (!parsedData) return;

    onApply(parsedData, {
      includeIdentity,
      includeWork,
      includeContacts,
      includeAddresses,
      includeSocials,
      includeBio,
      includeAvatar,
      mergeMode,
    });

    onOpenChange(false);
    resetState();
    appToast.success(t("profiles.vcard.autofill_applied"));
  };

  const phoneCount = (parsedData?.contact_details || []).filter(
    (c) => c.category === "phone" || c.category === "whatsapp",
  ).length;
  const emailCount = (parsedData?.contact_details || []).filter(
    (c) => c.category === "email",
  ).length;
  const addressCount = (parsedData?.addresses || []).length;
  const socialCount = (parsedData?.social_links || []).length;

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) resetState();
        onOpenChange(next);
      }}
    >
      <DialogContent className="border-border max-h-[90vh] gap-0 overflow-y-auto rounded-2xl p-0 shadow-2xl sm:max-w-2xl">
        {/* Header with decorative gradient banner */}
        <div className="from-primary/15 via-primary/5 to-background border-border/60 relative border-b bg-gradient-to-br p-6">
          <div className="flex items-start gap-4">
            <div className="bg-primary/10 border-primary/20 text-primary flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border shadow-xs">
              <FileCode2 className="h-6 w-6" />
            </div>
            <div className="flex-1 space-y-1">
              <div className="flex items-center gap-2">
                <DialogTitle className="text-lg font-bold">
                  {t("profiles.vcard.dialog_title")}
                </DialogTitle>
                <Badge
                  variant="outline"
                  className="bg-primary/10 text-primary border-primary/30 px-2 py-0.5 font-mono text-[10px]"
                >
                  vCard 3.0 / 4.0
                </Badge>
              </div>
              <DialogDescription className="text-muted-foreground text-xs leading-relaxed">
                {t("profiles.vcard.dialog_desc")}
              </DialogDescription>
            </div>
          </div>
        </div>

        <div className="space-y-6 p-6">
          {!parsedData ? (
            /* Upload & Dropzone Step */
            <div className="space-y-4">
              <div
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                onClick={() => fileInputRef.current?.click()}
                className={cn(
                  "flex cursor-pointer flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed p-8 text-center transition-all duration-200",
                  isDragging
                    ? "border-primary bg-primary/10 scale-[0.99]"
                    : "border-border/80 hover:border-primary/60 hover:bg-muted/30 bg-muted/10",
                  isLoading && "pointer-events-none opacity-60",
                )}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".vcf,.vcard"
                  onChange={handleInputChange}
                  className="hidden"
                />

                {isLoading ? (
                  <div className="flex flex-col items-center space-y-3 py-4">
                    <Loader2 className="text-primary h-10 w-10 animate-spin" />
                    <div className="space-y-1 text-center">
                      <p className="text-sm font-semibold">
                        {t("profiles.vcard.parsing_in_progress")}
                      </p>
                      <p className="text-muted-foreground text-xs">
                        {fileName}
                      </p>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="bg-primary/10 text-primary flex h-14 w-14 items-center justify-center rounded-full shadow-inner">
                      <UploadCloud className="h-7 w-7" />
                    </div>
                    <div className="space-y-1">
                      <p className="text-foreground text-sm font-bold">
                        {t("profiles.vcard.dropzone_cta")}
                      </p>
                      <p className="text-muted-foreground text-xs">
                        {t("profiles.vcard.dropzone_subtext")}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 pt-2">
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="pointer-events-none rounded-lg text-xs font-semibold shadow-xs"
                      >
                        <FileText className="mr-1.5 h-3.5 w-3.5" />
                        {t("profiles.vcard.browse_file")}
                      </Button>
                    </div>
                  </>
                )}
              </div>

              {/* Tips & Supported formats */}
              <div className="grid grid-cols-1 gap-3 text-xs sm:grid-cols-2">
                <div className="bg-muted/30 border-border/50 space-y-1 rounded-xl border p-3">
                  <div className="text-foreground flex items-center gap-1.5 font-semibold">
                    <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
                    <span>{t("profiles.vcard.tip_apple_title")}</span>
                  </div>
                  <p className="text-muted-foreground text-[11px]">
                    {t("profiles.vcard.tip_apple_desc")}
                  </p>
                </div>
                <div className="bg-muted/30 border-border/50 space-y-1 rounded-xl border p-3">
                  <div className="text-foreground flex items-center gap-1.5 font-semibold">
                    <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
                    <span>{t("profiles.vcard.tip_google_title")}</span>
                  </div>
                  <p className="text-muted-foreground text-[11px]">
                    {t("profiles.vcard.tip_google_desc")}
                  </p>
                </div>
              </div>
            </div>
          ) : (
            /* Parsed Result & Field Selection Step */
            <div className="animate-in fade-in slide-in-from-bottom-2 space-y-6 duration-300">
              {/* Profile Card Header Preview */}
              <div className="border-primary/20 bg-primary/5 flex flex-col items-start justify-between gap-4 rounded-2xl border p-4 sm:flex-row sm:items-center">
                <div className="flex items-center gap-3.5">
                  <Avatar className="border-primary/20 bg-background h-14 w-14 rounded-xl border-2 shadow-sm">
                    {parsedData.avatar_url || parsedData.avatar_data_url ? (
                      <AvatarImage
                        src={
                          parsedData.avatar_url || parsedData.avatar_data_url
                        }
                        alt="Avatar"
                        className="object-cover"
                      />
                    ) : null}
                    <AvatarFallback className="text-primary rounded-xl font-bold">
                      {parsedData.first_name?.[0] ||
                        parsedData.display_name?.[0] ||
                        "U"}
                    </AvatarFallback>
                  </Avatar>
                  <div className="space-y-0.5">
                    <div className="text-foreground flex items-center gap-2 text-base font-bold">
                      <span>
                        {parsedData.display_name ||
                          `${parsedData.first_name || ""} ${parsedData.last_name || ""}`.trim() ||
                          t("profiles.preview.untitled")}
                      </span>
                      {parsedData.nickname && (
                        <Badge
                          variant="secondary"
                          className="px-1.5 py-0 text-[10px]"
                        >
                          {parsedData.nickname}
                        </Badge>
                      )}
                    </div>
                    <p className="text-muted-foreground text-xs">
                      {[parsedData.title, parsedData.company_name]
                        .filter(Boolean)
                        .join(" • ") || t("profiles.preview.no_title")}
                    </p>
                  </div>
                </div>

                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={resetState}
                  className="h-8 shrink-0 gap-1.5 text-xs"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  <span>{t("profiles.vcard.change_file")}</span>
                </Button>
              </div>

              {/* Detected fields overview chips */}
              <div className="flex flex-wrap gap-2 text-xs">
                {phoneCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 px-2.5 py-1">
                    <Phone className="h-3 w-3 text-emerald-500" />
                    <span>
                      {phoneCount} {t("profiles.vcard.badge_phones")}
                    </span>
                  </Badge>
                )}
                {emailCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 px-2.5 py-1">
                    <Mail className="h-3 w-3 text-blue-500" />
                    <span>
                      {emailCount} {t("profiles.vcard.badge_emails")}
                    </span>
                  </Badge>
                )}
                {addressCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 px-2.5 py-1">
                    <MapPin className="h-3 w-3 text-rose-500" />
                    <span>
                      {addressCount} {t("profiles.vcard.badge_addresses")}
                    </span>
                  </Badge>
                )}
                {socialCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 px-2.5 py-1">
                    <Share2 className="h-3 w-3 text-indigo-500" />
                    <span>
                      {socialCount} {t("profiles.vcard.badge_socials")}
                    </span>
                  </Badge>
                )}
                {parsedData.birthday && (
                  <Badge variant="secondary" className="gap-1.5 px-2.5 py-1">
                    <Calendar className="h-3 w-3 text-amber-500" />
                    <span>{parsedData.birthday}</span>
                  </Badge>
                )}
              </div>

              {/* Field Selectors & Controls */}
              <div className="space-y-3">
                <div className="text-muted-foreground flex items-center justify-between text-xs font-semibold tracking-wider uppercase">
                  <span className="flex items-center gap-1.5">
                    <Sliders className="h-3.5 w-3.5" />
                    {t("profiles.vcard.select_fields_title")}
                  </span>
                  <span className="text-muted-foreground text-[11px] lowercase">
                    {t("profiles.vcard.toggle_hint")}
                  </span>
                </div>

                <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                  {/* Identity */}
                  <label className="border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center justify-between rounded-xl border p-3 text-xs transition-colors">
                    <div className="flex items-center gap-2.5">
                      <User className="text-primary h-4 w-4" />
                      <div>
                        <div className="font-semibold">
                          {t("profiles.tabs.identity")}
                        </div>
                        <div className="text-muted-foreground text-[11px]">
                          {parsedData.first_name} {parsedData.last_name}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={includeIdentity}
                      onCheckedChange={setIncludeIdentity}
                    />
                  </label>

                  {/* Work */}
                  <label className="border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center justify-between rounded-xl border p-3 text-xs transition-colors">
                    <div className="flex items-center gap-2.5">
                      <Briefcase className="h-4 w-4 text-amber-500" />
                      <div>
                        <div className="font-semibold">
                          {t("profiles.sections.work")}
                        </div>
                        <div className="text-muted-foreground max-w-[140px] truncate text-[11px]">
                          {parsedData.company_name || parsedData.title || "-"}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={includeWork}
                      onCheckedChange={setIncludeWork}
                    />
                  </label>

                  {/* Contacts */}
                  <label className="border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center justify-between rounded-xl border p-3 text-xs transition-colors">
                    <div className="flex items-center gap-2.5">
                      <Phone className="h-4 w-4 text-emerald-500" />
                      <div>
                        <div className="font-semibold">
                          {t("profiles.tabs.contacts")}
                        </div>
                        <div className="text-muted-foreground text-[11px]">
                          {phoneCount + emailCount}{" "}
                          {t("profiles.vcard.items_found")}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={includeContacts}
                      onCheckedChange={setIncludeContacts}
                    />
                  </label>

                  {/* Addresses */}
                  <label className="border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center justify-between rounded-xl border p-3 text-xs transition-colors">
                    <div className="flex items-center gap-2.5">
                      <MapPin className="h-4 w-4 text-rose-500" />
                      <div>
                        <div className="font-semibold">
                          {t("profiles.tabs.addresses")}
                        </div>
                        <div className="text-muted-foreground text-[11px]">
                          {addressCount} {t("profiles.vcard.items_found")}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={includeAddresses}
                      onCheckedChange={setIncludeAddresses}
                    />
                  </label>

                  {/* Socials */}
                  <label className="border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center justify-between rounded-xl border p-3 text-xs transition-colors">
                    <div className="flex items-center gap-2.5">
                      <Share2 className="h-4 w-4 text-indigo-500" />
                      <div>
                        <div className="font-semibold">
                          {t("profiles.tabs.social")}
                        </div>
                        <div className="text-muted-foreground text-[11px]">
                          {socialCount} {t("profiles.vcard.items_found")}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={includeSocials}
                      onCheckedChange={setIncludeSocials}
                    />
                  </label>

                  {/* Avatar / Photo */}
                  {(parsedData.avatar_url || parsedData.avatar_data_url) && (
                    <label className="border-border/70 bg-muted/20 hover:bg-muted/40 flex cursor-pointer items-center justify-between rounded-xl border p-3 text-xs transition-colors">
                      <div className="flex items-center gap-2.5">
                        <Sparkles className="text-primary h-4 w-4" />
                        <div>
                          <div className="font-semibold">
                            {t("profiles.fields.avatar")}
                          </div>
                          <div className="text-muted-foreground text-[11px]">
                            {t("profiles.vcard.embedded_photo")}
                          </div>
                        </div>
                      </div>
                      <Switch
                        checked={includeAvatar}
                        onCheckedChange={setIncludeAvatar}
                      />
                    </label>
                  )}
                </div>
              </div>

              {/* Merge vs Replace Mode Option */}
              <div className="bg-muted/30 border-border flex items-center justify-between gap-4 rounded-xl border p-3.5 text-xs">
                <div className="space-y-0.5">
                  <div className="text-foreground font-bold">
                    {t("profiles.vcard.merge_mode_title")}
                  </div>
                  <p className="text-muted-foreground text-[11px]">
                    {mergeMode === "merge"
                      ? t("profiles.vcard.merge_mode_merge_desc")
                      : t("profiles.vcard.merge_mode_replace_desc")}
                  </p>
                </div>

                <div className="bg-background border-border flex shrink-0 items-center gap-1 rounded-lg border p-1">
                  <button
                    type="button"
                    onClick={() => setMergeMode("replace")}
                    className={cn(
                      "rounded-md px-2.5 py-1 text-[11px] font-semibold transition-colors",
                      mergeMode === "replace"
                        ? "bg-primary text-primary-foreground shadow-xs"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                  >
                    {t("profiles.vcard.replace")}
                  </button>
                  <button
                    type="button"
                    onClick={() => setMergeMode("merge")}
                    className={cn(
                      "rounded-md px-2.5 py-1 text-[11px] font-semibold transition-colors",
                      mergeMode === "merge"
                        ? "bg-primary text-primary-foreground shadow-xs"
                        : "text-muted-foreground hover:text-foreground",
                    )}
                  >
                    {t("profiles.vcard.merge")}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="border-border bg-muted/20 flex items-center justify-between border-t p-4">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => {
              resetState();
              onOpenChange(false);
            }}
            className="text-xs"
          >
            {t("profiles.actions.cancel")}
          </Button>

          {parsedData && (
            <Button
              type="button"
              size="sm"
              onClick={handleConfirm}
              className="gap-2 px-4 text-xs font-semibold"
            >
              <Sparkles className="h-3.5 w-3.5" />
              <span>{t("profiles.vcard.apply_autofill")}</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
