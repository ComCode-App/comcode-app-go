"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useMemo, useRef, useState } from "react";
import {
  Copy,
  Download,
  Film,
  Layers,
  Maximize2,
  Minimize2,
  Paintbrush,
  Pause,
  Play,
  RotateCcw,
  Shuffle,
  Sparkles,
  Upload,
  Save,
} from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
  PremiumLockIcon,
  PremiumNotice,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { canManageAccount } from "@/lib/auth/types";
import { cn } from "@/lib/utils";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";

import { SHADER_REGISTRY } from "../data/shaders-registry";
import { isPremiumBackground } from "../lib/premium";
import {
  createDefaultDocument,
  generateRandomHex,
  getRandomColorPalette,
} from "../lib/defaults";
import {
  downloadJson,
  parseBackgroundDocument,
  stringifyBackground,
} from "../lib/json";
import type {
  AspectRatioType,
  BackgroundDocument,
  BackgroundKind,
  ShaderBackground,
} from "../types";
import { backgroundKeys, backgroundsService } from "../services/backgrounds.service";
import { BackgroundViewer } from "./background-viewer";
import { SavedBackgroundsPanel } from "./saved-backgrounds-panel";
import {
  GradientEditor,
  SolidEditor,
  VideoEditor,
} from "./kind-editors";
import { ShaderEditor } from "./shader-editor";

const KINDS: {
  id: BackgroundKind;
  icon: typeof Paintbrush;
  labelKey: string;
}[] = [
  { id: "solid", icon: Paintbrush, labelKey: "background_studio.kinds.solid" },
  {
    id: "gradient",
    icon: Layers,
    labelKey: "background_studio.kinds.gradient",
  },
  { id: "video", icon: Film, labelKey: "background_studio.kinds.video" },
  {
    id: "shader",
    icon: Sparkles,
    labelKey: "background_studio.kinds.shader",
  },
];

const ASPECT_LABEL_KEYS: Record<AspectRatioType, string> = {
  responsive: "background_studio.aspect.responsive",
  "16:9": "background_studio.aspect.16_9",
  "9:16": "background_studio.aspect.9_16",
  "1:1": "background_studio.aspect.1_1",
  "4:3": "background_studio.aspect.4_3",
};

function switchKind(
  current: BackgroundDocument,
  kind: BackgroundKind,
): BackgroundDocument {
  if (current.kind === kind) return current;
  const next = createDefaultDocument(kind);
  return {
    ...next,
    id: current.id,
    name: current.name,
    aspectRatio: current.aspectRatio,
    metadata: current.metadata,
  };
}

function shuffleDocument(doc: BackgroundDocument): BackgroundDocument {
  if (doc.kind === "solid") {
    return { ...doc, params: { color: generateRandomHex() } };
  }
  if (doc.kind === "gradient") {
    return {
      ...doc,
      params: {
        ...doc.params,
        colors: getRandomColorPalette(),
        angle: Math.floor(Math.random() * 360),
      },
    };
  }
  if (doc.kind === "shader") {
    const entry = SHADER_REGISTRY[doc.shaderType];
    if (!entry) return doc;
    const updated: Record<string, unknown> = { ...doc.params };
    const palette = getRandomColorPalette();
    if ("colors" in updated && Array.isArray(updated.colors)) {
      updated.colors = palette;
    }
    if ("colorFront" in updated) updated.colorFront = generateRandomHex();
    if ("colorBack" in updated) {
      updated.colorBack = Math.random() > 0.5 ? "#000000" : "#111827";
    }
    if ("colorTint" in updated) updated.colorTint = generateRandomHex();
    entry.params.forEach((p) => {
      if (
        p.type === "number" &&
        p.min !== undefined &&
        p.max !== undefined &&
        p.name !== "frame"
      ) {
        const rand = p.min + Math.random() * (p.max - p.min);
        updated[p.name] =
          p.step && p.step >= 1
            ? Math.round(rand)
            : Number(rand.toFixed(2));
      }
    });
    return { ...doc, params: updated };
  }
  return doc;
}

export function BackgroundStudioPage() {
  const { t } = useLocale();
  const [doc, setDoc] = useState<BackgroundDocument>(() =>
    createDefaultDocument("shader"),
  );
  const [fullscreen, setFullscreen] = useState(false);
  const [paused, setPaused] = useState(false);
  const [importText, setImportText] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  // Saved design the editor is linked to (null = new / preset / imported).
  const [savedUuid, setSavedUuid] = useState<string | null>(null);
  const { user } = useAuth();
  const canSave = canManageAccount(user);
  const queryClient = useQueryClient();
  // Animated designs (shaders with motion, video) can be designed on any plan but saved only
  // with design.premium (TEC-947); the API refuses them otherwise.
  const allowPremium = useDesignPremium();
  const saveLocked = !allowPremium && isPremiumBackground(doc);

  const save = useMutation({
    mutationFn: ({ asNew }: { asNew: boolean }) =>
      savedUuid && !asNew
        ? backgroundsService.update(savedUuid, doc.name.trim(), doc)
        : backgroundsService.create(doc.name.trim(), doc),
    onSuccess: (saved) => {
      setSavedUuid(saved.uuid);
      void queryClient.invalidateQueries({ queryKey: backgroundKeys.list });
      toast.success(t("background_studio.toast.saved"));
    },
  });
  const handleSave = (asNew: boolean) => {
    if (!doc.name.trim()) {
      toast.error(t("background_studio.name_required"));
      return;
    }
    save.mutate({ asNew });
  };

  const jsonPreview = useMemo(() => stringifyBackground(doc), [doc]);

  const setKind = useCallback((kind: BackgroundKind) => {
    setDoc((prev) => switchKind(prev, kind));
  }, []);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(stringifyBackground(doc));
    toast.success(t("background_studio.toast.copied"));
  };

  const handleDownload = () => {
    downloadJson(doc);
    toast.success(t("background_studio.toast.downloaded"));
  };

  const handleImportText = () => {
    try {
      const parsed = parseBackgroundDocument(JSON.parse(importText));
      setDoc(parsed);
      toast.success(t("background_studio.toast.imported"));
    } catch {
      toast.error(t("background_studio.toast.invalid_json"));
    }
  };

  const handleFileImport = async (file: File) => {
    try {
      const text = await file.text();
      const parsed = parseBackgroundDocument(JSON.parse(text));
      setDoc(parsed);
      setImportText(stringifyBackground(parsed));
      toast.success(t("background_studio.toast.imported"));
    } catch {
      toast.error(t("background_studio.toast.invalid_json"));
    }
  };

  const handleReset = () => {
    setDoc(createDefaultDocument(doc.kind));
    toast.success(t("background_studio.toast.reset"));
  };

  const handleShuffle = () => {
    setDoc((prev) => shuffleDocument(prev));
    toast.success(t("background_studio.toast.shuffled"));
  };

  return (
    <div
      className={cn(
        "flex flex-col gap-6",
        fullscreen && "fixed inset-0 z-50 bg-background p-4",
      )}
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("background_studio.title")}
            </h1>
            <Badge variant="secondary">{t("common.beta")}</Badge>
          </div>
          <p className="max-w-2xl text-sm text-muted-foreground">
            {t("background_studio.description")}
          </p>
          {canSave && saveLocked ? <PremiumNotice /> : null}
        </div>
        <div className="flex flex-wrap gap-2">
          {canSave ? (
            <>
              <Button type="button" size="sm" onClick={() => handleSave(false)} disabled={save.isPending || saveLocked}>
                <Save className="size-4" />
                {savedUuid ? t("background_studio.actions.save_changes") : t("background_studio.actions.save")}
              </Button>
              {savedUuid ? (
                <Button type="button" variant="outline" size="sm" onClick={() => handleSave(true)} disabled={save.isPending || saveLocked}>
                  {t("background_studio.actions.save_as_new")}
                </Button>
              ) : null}
            </>
          ) : null}
          <Button type="button" variant="outline" size="sm" onClick={handleShuffle}>
            <Shuffle className="size-4" />
            {t("background_studio.actions.shuffle")}
          </Button>
          <Button type="button" variant="outline" size="sm" onClick={handleReset}>
            <RotateCcw className="size-4" />
            {t("background_studio.actions.reset")}
          </Button>
          <Button type="button" variant="outline" size="sm" onClick={handleCopy}>
            <Copy className="size-4" />
            {t("background_studio.actions.copy_json")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleDownload}
          >
            <Download className="size-4" />
            {t("background_studio.actions.download_json")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setFullscreen((v) => !v)}
          >
            {fullscreen ? (
              <Minimize2 className="size-4" />
            ) : (
              <Maximize2 className="size-4" />
            )}
            {fullscreen
              ? t("background_studio.actions.exit_fullscreen")
              : t("background_studio.actions.fullscreen")}
          </Button>
        </div>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.4fr)_minmax(320px,0.9fr)]">
        <div className="space-y-4">
          <div className="relative">
            <BackgroundViewer
              doc={doc}
              fullscreen={fullscreen}
              paused={paused}
            />
            {(doc.kind === "shader" || doc.kind === "video") && (
              <Button
                type="button"
                size="icon"
                variant="secondary"
                className="absolute right-3 bottom-3"
                onClick={() => setPaused((v) => !v)}
              >
                {paused ? <Play className="size-4" /> : <Pause className="size-4" />}
              </Button>
            )}
          </div>

          <div className="flex flex-wrap gap-2">
            {KINDS.map(({ id, icon: Icon, labelKey }) => (
              <Button
                key={id}
                type="button"
                size="sm"
                variant={doc.kind === id ? "default" : "outline"}
                onClick={() => setKind(id)}
              >
                <Icon className="size-4" />
                {t(labelKey)}
                {id === "video" && !allowPremium ? <PremiumLockIcon className="text-current" /> : null}
              </Button>
            ))}
          </div>

          <SavedBackgroundsPanel
            activeUuid={savedUuid}
            canEdit={canSave}
            onLoad={(loaded, item) => {
              setDoc(loaded);
              setSavedUuid(item.is_preset ? null : item.uuid);
            }}
            onDeleted={(uuid) => setSavedUuid((cur) => (cur === uuid ? null : cur))}
          />
        </div>

        <div className="space-y-4 rounded-xl border bg-card p-4">
          <div className="space-y-2">
            <Label>{t("background_studio.fields.name")}</Label>
            <Input
              value={doc.name}
              onChange={(e) => setDoc({ ...doc, name: e.target.value })}
            />
          </div>

          <div className="space-y-2">
            <Label>{t("background_studio.fields.aspect_ratio")}</Label>
            <Select
              value={doc.aspectRatio ?? "responsive"}
              onValueChange={(value) =>
                setDoc({
                  ...doc,
                  aspectRatio: value as AspectRatioType,
                })
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(Object.keys(ASPECT_LABEL_KEYS) as AspectRatioType[]).map(
                  (a) => (
                    <SelectItem key={a} value={a}>
                      {t(ASPECT_LABEL_KEYS[a])}
                    </SelectItem>
                  ),
                )}
              </SelectContent>
            </Select>
          </div>

          <Tabs defaultValue="params">
            <TabsList className="w-full">
              <TabsTrigger value="params" className="flex-1">
                {t("background_studio.tabs.params")}
              </TabsTrigger>
              <TabsTrigger value="json" className="flex-1">
                {t("background_studio.tabs.json")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="params" className="mt-4 space-y-4">
              {doc.kind === "solid" && (
                <SolidEditor doc={doc} onChange={setDoc} />
              )}
              {doc.kind === "gradient" && (
                <GradientEditor doc={doc} onChange={setDoc} />
              )}
              {doc.kind === "video" && (
                <VideoEditor doc={doc} onChange={setDoc} />
              )}
              {doc.kind === "shader" && (
                <ShaderEditor
                  doc={doc as ShaderBackground}
                  onChange={setDoc}
                />
              )}
            </TabsContent>

            <TabsContent value="json" className="mt-4 space-y-3">
              <Textarea
                value={jsonPreview}
                readOnly
                className="min-h-[280px] font-mono text-xs"
              />
              <div className="space-y-2">
                <Label>{t("background_studio.json.import_label")}</Label>
                <Textarea
                  value={importText}
                  onChange={(e) => setImportText(e.target.value)}
                  placeholder='{ "version": "1.0", "kind": "shader", ... }'
                  className="min-h-[140px] font-mono text-xs"
                />
                <div className="flex flex-wrap gap-2">
                  <Button type="button" size="sm" onClick={handleImportText}>
                    <Upload className="size-4" />
                    {t("background_studio.actions.import_json")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => fileRef.current?.click()}
                  >
                    {t("background_studio.actions.import_file")}
                  </Button>
                  <input
                    ref={fileRef}
                    type="file"
                    accept="application/json,.json"
                    className="hidden"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) void handleFileImport(file);
                      e.target.value = "";
                    }}
                  />
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  );
}
