"use client";

import { useLocale } from "@/providers/locale-provider";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Slider } from "@/components/ui/slider";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Plus, Trash2 } from "lucide-react";

import { generateRandomHex } from "../lib/defaults";
import type {
  GradientBackground,
  SolidBackground,
  VideoBackground,
} from "../types";

type SolidEditorProps = {
  doc: SolidBackground;
  onChange: (next: SolidBackground) => void;
};

export function SolidEditor({ doc, onChange }: SolidEditorProps) {
  const { t } = useLocale();
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>{t("background_studio.solid.color")}</Label>
        <div className="flex items-center gap-3">
          <input
            type="color"
            value={doc.params.color}
            onChange={(e) =>
              onChange({
                ...doc,
                params: { ...doc.params, color: e.target.value },
              })
            }
            className="h-10 w-14 cursor-pointer rounded border bg-transparent p-1"
          />
          <Input
            value={doc.params.color}
            onChange={(e) =>
              onChange({
                ...doc,
                params: { ...doc.params, color: e.target.value },
              })
            }
            className="font-mono text-sm"
          />
        </div>
      </div>
    </div>
  );
}

type GradientEditorProps = {
  doc: GradientBackground;
  onChange: (next: GradientBackground) => void;
};

export function GradientEditor({ doc, onChange }: GradientEditorProps) {
  const { t } = useLocale();

  const updateColors = (colors: string[]) =>
    onChange({ ...doc, params: { ...doc.params, colors } });

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <Label>{t("background_studio.gradient.type")}</Label>
        <Select
          value={doc.params.type}
          onValueChange={(value) =>
            onChange({
              ...doc,
              params: {
                ...doc.params,
                type: value === "radial" ? "radial" : "linear",
              },
            })
          }
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="linear">
              {t("background_studio.gradient.linear")}
            </SelectItem>
            <SelectItem value="radial">
              {t("background_studio.gradient.radial")}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      {doc.params.type === "linear" && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>{t("background_studio.gradient.angle")}</Label>
            <span className="text-muted-foreground text-xs">
              {doc.params.angle ?? 135}°
            </span>
          </div>
          <Slider
            min={0}
            max={360}
            step={1}
            value={[doc.params.angle ?? 135]}
            onValueChange={([angle]) =>
              onChange({ ...doc, params: { ...doc.params, angle } })
            }
          />
        </div>
      )}

      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>{t("background_studio.gradient.colors")}</Label>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={doc.params.colors.length >= 8}
            onClick={() =>
              updateColors([...doc.params.colors, generateRandomHex()])
            }
          >
            <Plus className="size-3.5" />
            {t("background_studio.gradient.add_color")}
          </Button>
        </div>
        <div className="space-y-2">
          {doc.params.colors.map((color, index) => (
            <div key={`${index}-${color}`} className="flex items-center gap-2">
              <input
                type="color"
                value={color}
                onChange={(e) => {
                  const next = [...doc.params.colors];
                  next[index] = e.target.value;
                  updateColors(next);
                }}
                className="h-9 w-12 cursor-pointer rounded border bg-transparent p-1"
              />
              <Input
                value={color}
                onChange={(e) => {
                  const next = [...doc.params.colors];
                  next[index] = e.target.value;
                  updateColors(next);
                }}
                className="font-mono text-sm"
              />
              <Button
                type="button"
                size="icon"
                variant="ghost"
                disabled={doc.params.colors.length <= 2}
                onClick={() =>
                  updateColors(doc.params.colors.filter((_, i) => i !== index))
                }
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

type VideoEditorProps = {
  doc: VideoBackground;
  onChange: (next: VideoBackground) => void;
};

export function VideoEditor({ doc, onChange }: VideoEditorProps) {
  const { t } = useLocale();

  const setParam = <K extends keyof VideoBackground["params"]>(
    key: K,
    value: VideoBackground["params"][K],
  ) => onChange({ ...doc, params: { ...doc.params, [key]: value } });

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label>{t("background_studio.video.src")}</Label>
        <Input
          value={doc.params.src}
          placeholder="https://…/video.mp4"
          onChange={(e) => setParam("src", e.target.value)}
        />
        <p className="text-muted-foreground text-xs">
          {t("background_studio.video.src_hint")}
        </p>
      </div>

      <div className="space-y-2">
        <Label>{t("background_studio.video.poster")}</Label>
        <Input
          value={doc.params.poster ?? ""}
          placeholder="https://…/poster.jpg"
          onChange={(e) => setParam("poster", e.target.value || undefined)}
        />
      </div>

      <div className="space-y-2">
        <Label>{t("background_studio.video.object_fit")}</Label>
        <Select
          value={doc.params.objectFit ?? "cover"}
          onValueChange={(value) =>
            setParam(
              "objectFit",
              value === "contain" || value === "fill" ? value : "cover",
            )
          }
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="cover">cover</SelectItem>
            <SelectItem value="contain">contain</SelectItem>
            <SelectItem value="fill">fill</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        {(
          [
            ["muted", t("background_studio.video.muted")],
            ["loop", t("background_studio.video.loop")],
            ["autoplay", t("background_studio.video.autoplay")],
          ] as const
        ).map(([key, label]) => (
          <label
            key={key}
            className="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm"
          >
            <input
              type="checkbox"
              checked={doc.params[key] !== false}
              onChange={(e) => setParam(key, e.target.checked)}
              className="accent-primary size-4"
            />
            {label}
          </label>
        ))}
      </div>
    </div>
  );
}
