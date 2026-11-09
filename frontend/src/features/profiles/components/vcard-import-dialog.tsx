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
    if (!file.name.toLowerCase().endsWith(".vcf") && !file.name.toLowerCase().endsWith(".vcard")) {
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
      appToast.error(
        err instanceof Error ? err.message : t("profiles.vcard.parse_failed"),
      );
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
      <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto p-0 gap-0 border-border shadow-2xl rounded-2xl">
        {/* Header with decorative gradient banner */}
        <div className="relative p-6 bg-gradient-to-br from-primary/15 via-primary/5 to-background border-b border-border/60">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-2xl bg-primary/10 border border-primary/20 flex items-center justify-center text-primary shrink-0 shadow-xs">
              <FileCode2 className="w-6 h-6" />
            </div>
            <div className="space-y-1 flex-1">
              <div className="flex items-center gap-2">
                <DialogTitle className="text-lg font-bold">
                  {t("profiles.vcard.dialog_title")}
                </DialogTitle>
                <Badge
                  variant="outline"
                  className="bg-primary/10 text-primary border-primary/30 text-[10px] font-mono px-2 py-0.5"
                >
                  vCard 3.0 / 4.0
                </Badge>
              </div>
              <DialogDescription className="text-xs text-muted-foreground leading-relaxed">
                {t("profiles.vcard.dialog_desc")}
              </DialogDescription>
            </div>
          </div>
        </div>

        <div className="p-6 space-y-6">
          {!parsedData ? (
            /* Upload & Dropzone Step */
            <div className="space-y-4">
              <div
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
                onClick={() => fileInputRef.current?.click()}
                className={cn(
                  "border-2 border-dashed rounded-2xl p-8 text-center cursor-pointer transition-all duration-200 flex flex-col items-center justify-center gap-3",
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
                  <div className="space-y-3 py-4 flex flex-col items-center">
                    <Loader2 className="w-10 h-10 animate-spin text-primary" />
                    <div className="space-y-1 text-center">
                      <p className="text-sm font-semibold">
                        {t("profiles.vcard.parsing_in_progress")}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {fileName}
                      </p>
                    </div>
                  </div>
                ) : (
                  <>
                    <div className="w-14 h-14 rounded-full bg-primary/10 text-primary flex items-center justify-center shadow-inner">
                      <UploadCloud className="w-7 h-7" />
                    </div>
                    <div className="space-y-1">
                      <p className="text-sm font-bold text-foreground">
                        {t("profiles.vcard.dropzone_cta")}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {t("profiles.vcard.dropzone_subtext")}
                      </p>
                    </div>
                    <div className="pt-2 flex items-center gap-2">
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        className="text-xs font-semibold rounded-lg shadow-xs pointer-events-none"
                      >
                        <FileText className="w-3.5 h-3.5 mr-1.5" />
                        {t("profiles.vcard.browse_file")}
                      </Button>
                    </div>
                  </>
                )}
              </div>

              {/* Tips & Supported formats */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                <div className="p-3 rounded-xl bg-muted/30 border border-border/50 space-y-1">
                  <div className="font-semibold flex items-center gap-1.5 text-foreground">
                    <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
                    <span>{t("profiles.vcard.tip_apple_title")}</span>
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    {t("profiles.vcard.tip_apple_desc")}
                  </p>
                </div>
                <div className="p-3 rounded-xl bg-muted/30 border border-border/50 space-y-1">
                  <div className="font-semibold flex items-center gap-1.5 text-foreground">
                    <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
                    <span>{t("profiles.vcard.tip_google_title")}</span>
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    {t("profiles.vcard.tip_google_desc")}
                  </p>
                </div>
              </div>
            </div>
          ) : (
            /* Parsed Result & Field Selection Step */
            <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-300">
              {/* Profile Card Header Preview */}
              <div className="p-4 rounded-2xl border border-primary/20 bg-primary/5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                <div className="flex items-center gap-3.5">
                  <Avatar className="w-14 h-14 rounded-xl border-2 border-primary/20 bg-background shadow-sm">
                    {parsedData.avatar_url || parsedData.avatar_data_url ? (
                      <AvatarImage
                        src={parsedData.avatar_url || parsedData.avatar_data_url}
                        alt="Avatar"
                        className="object-cover"
                      />
                    ) : null}
                    <AvatarFallback className="rounded-xl font-bold text-primary">
                      {parsedData.first_name?.[0] || parsedData.display_name?.[0] || "U"}
                    </AvatarFallback>
                  </Avatar>
                  <div className="space-y-0.5">
                    <div className="font-bold text-base text-foreground flex items-center gap-2">
                      <span>
                        {parsedData.display_name ||
                          `${parsedData.first_name || ""} ${parsedData.last_name || ""}`.trim() ||
                          t("profiles.preview.untitled")}
                      </span>
                      {parsedData.nickname && (
                        <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                          {parsedData.nickname}
                        </Badge>
                      )}
                    </div>
                    <p className="text-xs text-muted-foreground">
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
                  className="text-xs gap-1.5 h-8 shrink-0"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                  <span>{t("profiles.vcard.change_file")}</span>
                </Button>
              </div>

              {/* Detected fields overview chips */}
              <div className="flex flex-wrap gap-2 text-xs">
                {phoneCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 py-1 px-2.5">
                    <Phone className="w-3 h-3 text-emerald-500" />
                    <span>{phoneCount} {t("profiles.vcard.badge_phones")}</span>
                  </Badge>
                )}
                {emailCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 py-1 px-2.5">
                    <Mail className="w-3 h-3 text-blue-500" />
                    <span>{emailCount} {t("profiles.vcard.badge_emails")}</span>
                  </Badge>
                )}
                {addressCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 py-1 px-2.5">
                    <MapPin className="w-3 h-3 text-rose-500" />
                    <span>{addressCount} {t("profiles.vcard.badge_addresses")}</span>
                  </Badge>
                )}
                {socialCount > 0 && (
                  <Badge variant="secondary" className="gap-1.5 py-1 px-2.5">
                    <Share2 className="w-3 h-3 text-indigo-500" />
                    <span>{socialCount} {t("profiles.vcard.badge_socials")}</span>
                  </Badge>
                )}
                {parsedData.birthday && (
                  <Badge variant="secondary" className="gap-1.5 py-1 px-2.5">
                    <Calendar className="w-3 h-3 text-amber-500" />
                    <span>{parsedData.birthday}</span>
                  </Badge>
                )}
              </div>

              {/* Field Selectors & Controls */}
              <div className="space-y-3">
                <div className="font-semibold text-xs uppercase tracking-wider text-muted-foreground flex items-center justify-between">
                  <span className="flex items-center gap-1.5">
                    <Sliders className="w-3.5 h-3.5" />
                    {t("profiles.vcard.select_fields_title")}
                  </span>
                  <span className="text-[11px] lowercase text-muted-foreground">
                    {t("profiles.vcard.toggle_hint")}
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  {/* Identity */}
                  <label className="flex items-center justify-between p-3 rounded-xl border border-border/70 bg-muted/20 hover:bg-muted/40 transition-colors cursor-pointer text-xs">
                    <div className="flex items-center gap-2.5">
                      <User className="w-4 h-4 text-primary" />
                      <div>
                        <div className="font-semibold">{t("profiles.tabs.identity")}</div>
                        <div className="text-[11px] text-muted-foreground">
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
                  <label className="flex items-center justify-between p-3 rounded-xl border border-border/70 bg-muted/20 hover:bg-muted/40 transition-colors cursor-pointer text-xs">
                    <div className="flex items-center gap-2.5">
                      <Briefcase className="w-4 h-4 text-amber-500" />
                      <div>
                        <div className="font-semibold">{t("profiles.sections.work")}</div>
                        <div className="text-[11px] text-muted-foreground truncate max-w-[140px]">
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
                  <label className="flex items-center justify-between p-3 rounded-xl border border-border/70 bg-muted/20 hover:bg-muted/40 transition-colors cursor-pointer text-xs">
                    <div className="flex items-center gap-2.5">
                      <Phone className="w-4 h-4 text-emerald-500" />
                      <div>
                        <div className="font-semibold">{t("profiles.tabs.contacts")}</div>
                        <div className="text-[11px] text-muted-foreground">
                          {phoneCount + emailCount} {t("profiles.vcard.items_found")}
                        </div>
                      </div>
                    </div>
                    <Switch
                      checked={includeContacts}
                      onCheckedChange={setIncludeContacts}
                    />
                  </label>

                  {/* Addresses */}
                  <label className="flex items-center justify-between p-3 rounded-xl border border-border/70 bg-muted/20 hover:bg-muted/40 transition-colors cursor-pointer text-xs">
                    <div className="flex items-center gap-2.5">
                      <MapPin className="w-4 h-4 text-rose-500" />
                      <div>
                        <div className="font-semibold">{t("profiles.tabs.addresses")}</div>
                        <div className="text-[11px] text-muted-foreground">
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
                  <label className="flex items-center justify-between p-3 rounded-xl border border-border/70 bg-muted/20 hover:bg-muted/40 transition-colors cursor-pointer text-xs">
                    <div className="flex items-center gap-2.5">
                      <Share2 className="w-4 h-4 text-indigo-500" />
                      <div>
                        <div className="font-semibold">{t("profiles.tabs.social")}</div>
                        <div className="text-[11px] text-muted-foreground">
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
                    <label className="flex items-center justify-between p-3 rounded-xl border border-border/70 bg-muted/20 hover:bg-muted/40 transition-colors cursor-pointer text-xs">
                      <div className="flex items-center gap-2.5">
                        <Sparkles className="w-4 h-4 text-primary" />
                        <div>
                          <div className="font-semibold">{t("profiles.fields.avatar")}</div>
                          <div className="text-[11px] text-muted-foreground">
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
              <div className="p-3.5 rounded-xl bg-muted/30 border border-border flex items-center justify-between gap-4 text-xs">
                <div className="space-y-0.5">
                  <div className="font-bold text-foreground">
                    {t("profiles.vcard.merge_mode_title")}
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    {mergeMode === "merge"
                      ? t("profiles.vcard.merge_mode_merge_desc")
                      : t("profiles.vcard.merge_mode_replace_desc")}
                  </p>
                </div>

                <div className="flex items-center gap-1 bg-background p-1 rounded-lg border border-border shrink-0">
                  <button
                    type="button"
                    onClick={() => setMergeMode("replace")}
                    className={cn(
                      "px-2.5 py-1 rounded-md font-semibold text-[11px] transition-colors",
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
                      "px-2.5 py-1 rounded-md font-semibold text-[11px] transition-colors",
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
        <div className="p-4 border-t border-border bg-muted/20 flex items-center justify-between">
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
              className="gap-2 text-xs font-semibold px-4"
            >
              <Sparkles className="w-3.5 h-3.5" />
              <span>{t("profiles.vcard.apply_autofill")}</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
