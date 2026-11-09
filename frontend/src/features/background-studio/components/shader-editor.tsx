"use client";

import { useMemo, useState } from "react";
import { Plus, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Slider } from "@/components/ui/slider";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  PremiumLockIcon,
  useDesignPremium,
} from "@/features/entitlements/components/premium-lock";
import { useLocale } from "@/providers/locale-provider";
import { cn } from "@/lib/utils";

import {
  SHADER_LIST,
  SHADER_REGISTRY,
  type ParamDefinition,
} from "../data/shaders-registry";
import type { ShaderType } from "../data/shader-types";
import { generateRandomHex } from "../lib/defaults";
import { isPremiumShader } from "../lib/premium";
import {
  getCategoryLabel,
  getParamLabel,
  getShaderDescription,
  getShaderLabel,
} from "../lib/shader-i18n";
import type { ShaderBackground } from "../types";

const CATEGORIES = [
  "all",
  "effects",
  "patterns",
  "noise",
  "imageFilters",
  "logo",
] as const;

type ShaderEditorProps = {
  doc: ShaderBackground;
  onChange: (next: ShaderBackground) => void;
};

function ParamControl({
  param,
  value,
  label,
  onChange,
}: {
  param: ParamDefinition;
  value: unknown;
  label: string;
  onChange: (value: unknown) => void;
}) {
  if (
    param.type === "number" &&
    param.min !== undefined &&
    param.max !== undefined
  ) {
    const num =
      typeof value === "number"
        ? value
        : Number(param.defaultValue ?? param.min);
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-2">
          <Label className="text-xs">{label}</Label>
          <span className="text-muted-foreground font-mono text-[11px]">
            {Number.isFinite(num) ? num : "—"}
          </span>
        </div>
        <Slider
          min={param.min}
          max={param.max}
          step={param.step ?? 0.01}
          value={[Number.isFinite(num) ? num : param.min]}
          onValueChange={([v]) => onChange(v)}
        />
      </div>
    );
  }

  if (param.type === "enum" && param.options) {
    return (
      <div className="space-y-2">
        <Label className="text-xs">{label}</Label>
        <Select
          value={String(value ?? param.options[0])}
          onValueChange={(v) => onChange(v)}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {param.options.map((opt) => (
              <SelectItem key={opt} value={opt}>
                {opt}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    );
  }

  if (param.isColor || (param.type === "string" && param.isColor)) {
    const color = typeof value === "string" ? value : "#000000";
    return (
      <div className="space-y-2">
        <Label className="text-xs">{label}</Label>
        <div className="flex items-center gap-2">
          <input
            type="color"
            value={color}
            onChange={(e) => onChange(e.target.value)}
            className="h-9 w-12 cursor-pointer rounded border bg-transparent p-1"
          />
          <Input
            value={color}
            onChange={(e) => onChange(e.target.value)}
            className="font-mono text-sm"
          />
        </div>
      </div>
    );
  }

  if (param.type === "string[]") {
    const colors = Array.isArray(value)
      ? value.filter((c): c is string => typeof c === "string")
      : [];
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label className="text-xs">{label}</Label>
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={colors.length >= 10}
            onClick={() => onChange([...colors, generateRandomHex()])}
          >
            <Plus className="size-3.5" />
          </Button>
        </div>
        <div className="space-y-2">
          {colors.map((color, index) => (
            <div key={`${index}-${color}`} className="flex items-center gap-2">
              <input
                type="color"
                value={color}
                onChange={(e) => {
                  const next = [...colors];
                  next[index] = e.target.value;
                  onChange(next);
                }}
                className="h-8 w-10 cursor-pointer rounded border bg-transparent p-0.5"
              />
              <Input
                value={color}
                onChange={(e) => {
                  const next = [...colors];
                  next[index] = e.target.value;
                  onChange(next);
                }}
                className="font-mono text-xs"
              />
              <Button
                type="button"
                size="icon"
                variant="ghost"
                disabled={colors.length <= 2}
                onClick={() => onChange(colors.filter((_, i) => i !== index))}
              >
                <Trash2 className="size-3.5" />
              </Button>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <Label className="text-xs">{label}</Label>
      <Input
        value={value == null ? "" : String(value)}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}

export function ShaderEditor({ doc, onChange }: ShaderEditorProps) {
  const { t, locale } = useLocale();
  const [category, setCategory] = useState<string>("all");
  const [search, setSearch] = useState("");
  // Animated shaders can be tried out on any plan; saving them needs design.premium (TEC-947).
  const allowPremium = useDesignPremium();

  const entry =
    SHADER_REGISTRY[doc.shaderType] ?? SHADER_REGISTRY["mesh-gradient"];

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return SHADER_LIST.filter((s) => {
      const catOk = category === "all" || s.category === category;
      if (!catOk) return false;
      if (!q) return true;
      return (
        s.id.includes(q) ||
        s.name.toLowerCase().includes(q) ||
        s.nameTr.toLowerCase().includes(q)
      );
    });
  }, [category, search]);

  const selectShader = (id: string) => {
    const next = SHADER_REGISTRY[id];
    if (!next) return;
    onChange({
      ...doc,
      shaderType: next.id as ShaderType,
      category: next.category,
      name: `${next.name} Background`,
      params: { ...next.defaultParams },
    });
  };

  const setParam = (key: string, value: unknown) => {
    onChange({
      ...doc,
      params: { ...doc.params, [key]: value },
    });
  };

  const customParams = entry.params.filter(
    (p) => p.category === "custom" || p.category === "color" || !p.category,
  );
  const motionParams = entry.params.filter((p) => p.category === "motion");
  const sizingParams = entry.params.filter((p) => p.category === "sizing");

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <Label>{t("background_studio.shader.search")}</Label>
        <Input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t("background_studio.shader.search_placeholder")}
        />
      </div>

      <div className="flex flex-wrap gap-1.5">
        {CATEGORIES.map((c) => (
          <Button
            key={c}
            type="button"
            size="sm"
            variant={category === c ? "default" : "outline"}
            onClick={() => setCategory(c)}
          >
            {getCategoryLabel(c, locale)}
          </Button>
        ))}
      </div>

      <ScrollArea className="h-44 rounded-lg border">
        <div className="grid gap-1 p-2 sm:grid-cols-2">
          {filtered.map((s) => (
            <button
              key={s.id}
              type="button"
              onClick={() => selectShader(s.id)}
              className={cn(
                "rounded-md px-2.5 py-2 text-left text-xs transition-colors",
                s.id === doc.shaderType
                  ? "bg-primary text-primary-foreground"
                  : "hover:bg-muted",
              )}
            >
              <div className="flex items-center justify-between gap-1 font-medium">
                <span className="truncate">{getShaderLabel(s.id, locale)}</span>
                {!allowPremium && isPremiumShader(s.id) ? (
                  <PremiumLockIcon
                    className={cn(
                      s.id === doc.shaderType && "text-primary-foreground/80",
                    )}
                  />
                ) : null}
              </div>
              <div
                className={cn(
                  "mt-0.5 line-clamp-1 text-[10px]",
                  s.id === doc.shaderType
                    ? "text-primary-foreground/80"
                    : "text-muted-foreground",
                )}
              >
                {s.id}
              </div>
            </button>
          ))}
        </div>
      </ScrollArea>

      <p className="text-muted-foreground text-xs">
        {getShaderDescription(doc.shaderType, locale)}
      </p>

      <div className="space-y-4">
        <h4 className="text-sm font-medium">
          {t("background_studio.shader.parameters")}
        </h4>
        {[...customParams, ...motionParams, ...sizingParams].map((param) => (
          <ParamControl
            key={param.name}
            param={param}
            value={doc.params[param.name]}
            label={getParamLabel(doc.shaderType, param.name, locale)}
            onChange={(v) => setParam(param.name, v)}
          />
        ))}
      </div>
    </div>
  );
}
