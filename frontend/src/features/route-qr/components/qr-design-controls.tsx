"use client";

import { PROFILE_COVER_ENABLED } from "@/config/profile-media";
import { FlipHorizontal2, FlipVertical2, Sparkles } from "lucide-react";
import { useId, useMemo } from "react";

import { Button } from "@/components/ui/button";
import { ColorPicker } from "@/components/ui/color-picker";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import type { ProfileSummary } from "@/features/profiles/services/profiles.service";
import { cn } from "@/lib/utils";
import { useLocale } from "@/providers/locale-provider";

import templatesJson from "../data/qr-templates.json";
import {
  BODY_SHAPES,
  EYE_BALL_SHAPES,
  EYE_FRAME_SHAPES,
  isHexColor,
  LOGO_SIZE_MAX,
  LOGO_SIZE_MIN,
  normalizeQrStyle,
  QUIET_ZONE_MAX,
  QUIET_ZONE_MIN,
  type QrEye,
  type QrStyle,
  type QrStyleFull,
  renderQrSvg,
} from "../lib/qr-engine";
import { svgDataUrl } from "../lib/qr-export";
import {
  applyTemplate,
  type EyeColorMode,
  eyeColorMode,
  matchesTemplate,
  mirroredEyes,
  type QrTemplate,
} from "../lib/qr-style";
import { QrShapePicker } from "./qr-shape-picker";
import { TooltipHint } from "@/components/ui/tooltip";

const TEMPLATES = templatesJson as QrTemplate[];

type Props = {
  style: QrStyleFull;
  set: (patch: Partial<QrStyle>) => void;
  canEdit: boolean;
  disabled: boolean;
  matrix: string[] | null;
  profiles: ProfileSummary[];
  suggestedLogoProfileUuid?: string;
  logoMedia: { avatar?: string; cover?: string };
  logoLoading: boolean;
  logoMissing: boolean;
};

export function QrDesignControls(props: Props) {
  const { t } = useLocale();
  const { canEdit, disabled } = props;
  return (
    <div className="min-w-0 space-y-3">
      {!canEdit ? (
        <p className="text-muted-foreground text-sm">
          {t("routes.qr.read_only")}
        </p>
      ) : null}
      <Tabs defaultValue="templates" className="min-w-0">
        <TabsList className="grid w-full grid-cols-4">
          <TabsTrigger value="templates">
            {t("routes.qr.tab_templates")}
          </TabsTrigger>
          <TabsTrigger value="shapes">{t("routes.qr.tab_shapes")}</TabsTrigger>
          <TabsTrigger value="colors">{t("routes.qr.tab_colors")}</TabsTrigger>
          <TabsTrigger value="logo">{t("routes.qr.tab_logo")}</TabsTrigger>
        </TabsList>
        <fieldset disabled={disabled} className="min-w-0">
          <TabsContent value="templates" className="pt-2">
            <TemplatesPanel {...props} />
          </TabsContent>
          <TabsContent value="shapes" className="space-y-5 pt-2">
            <ShapesPanel {...props} />
          </TabsContent>
          <TabsContent value="colors" className="space-y-5 pt-2">
            <ColorsPanel {...props} />
          </TabsContent>
          <TabsContent value="logo" className="space-y-4 pt-2">
            <LogoPanel {...props} />
          </TabsContent>
        </fieldset>
      </Tabs>
    </div>
  );
}

function TemplatesPanel({ style, set, matrix, disabled }: Props) {
  const { t } = useLocale();
  // Thumbnails use the route's own matrix, so each is exactly what applying it would show.
  const items = useMemo(
    () =>
      TEMPLATES.map((tpl) => {
        const applied = applyTemplate(style, tpl);
        const full = normalizeQrStyle(applied);
        return {
          tpl,
          applied,
          active: matchesTemplate(style, full),
          src: matrix
            ? svgDataUrl(
                renderQrSvg(matrix, { ...full, logo_profile_uuid: null }, 160),
              )
            : null,
        };
      }),
    [style, matrix],
  );
  return (
    <div className="space-y-2">
      <p className="text-muted-foreground text-xs">
        {t("routes.qr.templates_hint")}
      </p>
      <div className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-3 lg:grid-cols-4">
        {items.map(({ tpl, applied, active, src }) => (
          <button
            key={tpl.id}
            type="button"
            aria-pressed={active}
            disabled={disabled}
            onClick={() => set(applied)}
            className={cn(
              "bg-background flex flex-col items-center gap-1 rounded-lg border p-1.5 text-xs transition-colors disabled:cursor-not-allowed disabled:opacity-60",
              active
                ? "border-primary ring-primary/30 ring-2"
                : "hover:bg-muted",
            )}
          >
            {src ? (
              // eslint-disable-next-line @next/next/no-img-element -- data URL of the engine's SVG
              <img src={src} alt="" className="aspect-square w-full rounded" />
            ) : null}
            <span className="truncate">
              {t(`routes.qr.template_${tpl.id}`)}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

const EYE_INDEXES = [0, 1, 2] as const;

function ShapesPanel({ style, set, disabled }: Props) {
  const { t } = useLocale();
  const ids = useId();
  const setEye = (i: number, patch: Partial<QrEye>) =>
    set({ eyes: style.eyes.map((e, j) => (j === i ? { ...e, ...patch } : e)) });
  return (
    <>
      <div className="grid gap-1.5">
        <Label>{t("routes.qr.body_shape")}</Label>
        <QrShapePicker
          kind="body"
          shapes={BODY_SHAPES}
          value={style.body_shape}
          onChange={(body_shape) => set({ body_shape, module: "" })}
          disabled={disabled}
          label={t("routes.qr.body_shape")}
        />
      </div>
      <div className="grid gap-1.5">
        <Label>{t("routes.qr.eye_frame_shape")}</Label>
        <QrShapePicker
          kind="frame"
          shapes={EYE_FRAME_SHAPES}
          value={style.eye_frame_shape}
          onChange={(eye_frame_shape) => set({ eye_frame_shape, eye: "" })}
          disabled={disabled}
          label={t("routes.qr.eye_frame_shape")}
        />
      </div>
      <div className="grid gap-1.5">
        <Label>{t("routes.qr.eye_ball_shape")}</Label>
        <QrShapePicker
          kind="ball"
          shapes={EYE_BALL_SHAPES}
          value={style.eye_ball_shape}
          onChange={(eye_ball_shape) => set({ eye_ball_shape, eye: "" })}
          disabled={disabled}
          label={t("routes.qr.eye_ball_shape")}
        />
      </div>
      <div className="grid gap-2">
        <div className="flex items-center justify-between gap-2">
          <Label>{t("routes.qr.eye_flips")}</Label>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => set({ eyes: mirroredEyes(style.eyes) })}
          >
            <Sparkles className="size-3.5" />
            {t("routes.qr.eye_mirror")}
          </Button>
        </div>
        <div className="grid grid-cols-3 gap-2">
          {EYE_INDEXES.map((i) => (
            <div key={i} className="grid gap-1 rounded-md border p-2">
              <span className="text-muted-foreground text-xs">
                {t(`routes.qr.eye_name_${i}`)}
              </span>
              <div className="flex gap-1">
                <TooltipHint label={t("routes.qr.flip_h")}>
                  <Button
                    type="button"
                    size="icon"
                    variant={style.eyes[i].flip_h ? "default" : "outline"}
                    aria-pressed={style.eyes[i].flip_h}
                    aria-label={t("routes.qr.flip_h")}
                    onClick={() => setEye(i, { flip_h: !style.eyes[i].flip_h })}
                    className="size-8"
                  >
                    <FlipHorizontal2 className="size-4" />
                    <span className="sr-only">{t("routes.qr.flip_h")}</span>
                  </Button>
                </TooltipHint>
                <TooltipHint label={t("routes.qr.flip_v")}>
                  <Button
                    type="button"
                    size="icon"
                    variant={style.eyes[i].flip_v ? "default" : "outline"}
                    aria-pressed={style.eyes[i].flip_v}
                    aria-label={t("routes.qr.flip_v")}
                    onClick={() => setEye(i, { flip_v: !style.eyes[i].flip_v })}
                    className="size-8"
                  >
                    <FlipVertical2 className="size-4" />
                    <span className="sr-only">{t("routes.qr.flip_v")}</span>
                  </Button>
                </TooltipHint>
              </div>
            </div>
          ))}
        </div>
        <p className="text-muted-foreground text-xs">
          {t("routes.qr.eye_flips_hint")}
        </p>
      </div>
      <div className="grid gap-2">
        <div className="flex items-center justify-between">
          <Label htmlFor={`${ids}-quiet`}>{t("routes.qr.quiet_zone")}</Label>
          <span className="text-muted-foreground text-xs tabular-nums">
            {t("routes.qr.quiet_zone_value", { count: style.quiet_zone })}
          </span>
        </div>
        <Slider
          id={`${ids}-quiet`}
          min={QUIET_ZONE_MIN}
          max={QUIET_ZONE_MAX}
          step={1}
          value={[style.quiet_zone]}
          onValueChange={([v]) => set({ quiet_zone: v ?? QUIET_ZONE_MIN })}
          disabled={disabled}
        />
        <p className="text-muted-foreground text-xs">
          {t("routes.qr.quiet_zone_hint")}
        </p>
      </div>
    </>
  );
}

function ColorsPanel({ style, set, disabled }: Props) {
  const { t } = useLocale();
  const ids = useId();
  const gradient = style.fill_mode !== "solid";
  const mode = eyeColorMode(style.eyes);
  const setEyeColors = (next: EyeColorMode) => {
    const frame = style.eyes[0].frame_color || style.gradient_start;
    const ball = style.eyes[0].ball_color || style.foreground;
    if (next === "inherit")
      set({
        eyes: style.eyes.map((e) => ({
          ...e,
          frame_color: "",
          ball_color: "",
        })),
      });
    else
      set({
        eyes: style.eyes.map((e) => ({
          ...e,
          frame_color: frame,
          ball_color: ball,
        })),
      });
  };
  const setEye = (i: number | "all", patch: Partial<QrEye>) =>
    set({
      eyes: style.eyes.map((e, j) =>
        i === "all" || j === i ? { ...e, ...patch } : e,
      ),
    });
  return (
    <>
      <div className="grid gap-1.5">
        <Label>{t("routes.qr.fill_mode")}</Label>
        <ToggleGroup
          type="single"
          variant="outline"
          value={style.fill_mode}
          onValueChange={(v) => v && set({ fill_mode: v })}
          disabled={disabled}
          className="w-full"
        >
          {(["solid", "linear", "radial"] as const).map((m) => (
            <ToggleGroupItem key={m} value={m} className="flex-1">
              {t(`routes.qr.fill_${m}`)}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>
      {gradient ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <ColorField
            id={`${ids}-g1`}
            label={t("routes.qr.gradient_start")}
            value={style.gradient_start}
            onChange={(gradient_start) => set({ gradient_start })}
          />
          <ColorField
            id={`${ids}-g2`}
            label={t("routes.qr.gradient_end")}
            value={style.gradient_end}
            onChange={(gradient_end) => set({ gradient_end })}
          />
          {style.fill_mode === "linear" ? (
            <div className="grid gap-2 sm:col-span-2">
              <div className="flex items-center justify-between">
                <Label htmlFor={`${ids}-angle`}>
                  {t("routes.qr.gradient_angle")}
                </Label>
                <span className="text-muted-foreground text-xs tabular-nums">
                  {style.gradient_angle}°
                </span>
              </div>
              <Slider
                id={`${ids}-angle`}
                min={0}
                max={359}
                step={1}
                value={[style.gradient_angle]}
                onValueChange={([v]) => set({ gradient_angle: v ?? 0 })}
                disabled={disabled}
              />
            </div>
          ) : null}
          <SwitchRow
            id={`${ids}-geyes`}
            label={t("routes.qr.gradient_on_eyes")}
            checked={style.gradient_on_eyes}
            onChange={(gradient_on_eyes) => set({ gradient_on_eyes })}
            disabled={disabled}
          />
        </div>
      ) : null}
      <div className="grid gap-4 sm:grid-cols-2">
        <ColorField
          id={`${ids}-fg`}
          label={
            gradient
              ? t("routes.qr.foreground_eyes")
              : t("routes.qr.foreground")
          }
          value={style.foreground}
          onChange={(foreground) => set({ foreground })}
        />
        <ColorField
          id={`${ids}-bg`}
          label={t("routes.qr.background")}
          value={style.background}
          onChange={(background) => set({ background })}
          disabled={style.transparent_background}
        />
        <SwitchRow
          id={`${ids}-transparent`}
          label={t("routes.qr.transparent_background")}
          checked={style.transparent_background}
          onChange={(transparent_background) => set({ transparent_background })}
          disabled={disabled}
        />
      </div>
      <div className="grid gap-2">
        <Label>{t("routes.qr.eye_colors")}</Label>
        <Select
          value={mode}
          onValueChange={(v) => setEyeColors(v as EyeColorMode)}
          disabled={disabled}
        >
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(["inherit", "shared", "per_eye"] as const).map((m) => (
              <SelectItem key={m} value={m}>
                {t(`routes.qr.eye_colors_${m}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {mode === "shared" ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <ColorField
              id={`${ids}-ef`}
              label={t("routes.qr.eye_frame_color")}
              value={style.eyes[0].frame_color}
              onChange={(frame_color) => setEye("all", { frame_color })}
            />
            <ColorField
              id={`${ids}-eb`}
              label={t("routes.qr.eye_ball_color")}
              value={style.eyes[0].ball_color}
              onChange={(ball_color) => setEye("all", { ball_color })}
            />
          </div>
        ) : null}
        {mode === "per_eye"
          ? EYE_INDEXES.map((i) => (
              <div key={i} className="grid gap-2 rounded-md border p-2">
                <span className="text-muted-foreground text-xs">
                  {t(`routes.qr.eye_name_${i}`)}
                </span>
                <div className="grid gap-3 sm:grid-cols-2">
                  <ColorField
                    id={`${ids}-ef${i}`}
                    label={t("routes.qr.eye_frame_color")}
                    value={style.eyes[i].frame_color}
                    onChange={(frame_color) => setEye(i, { frame_color })}
                  />
                  <ColorField
                    id={`${ids}-eb${i}`}
                    label={t("routes.qr.eye_ball_color")}
                    value={style.eyes[i].ball_color}
                    onChange={(ball_color) => setEye(i, { ball_color })}
                  />
                </div>
              </div>
            ))
          : null}
        <p className="text-muted-foreground text-xs">
          {t("routes.qr.eye_colors_hint")}
        </p>
      </div>
    </>
  );
}

function LogoPanel({
  style,
  set,
  canEdit,
  disabled,
  profiles,
  suggestedLogoProfileUuid,
  logoMedia,
  logoLoading,
  logoMissing,
}: Props) {
  const { t } = useLocale();
  const ids = useId();
  const logoProfileUuid = style.logo_profile_uuid;
  const pickLogoProfile = (value: string) => {
    if (value === "none") {
      set({ logo_profile_uuid: null, logo_field: "" });
      return;
    }
    set({ logo_profile_uuid: value, logo_field: style.logo_field || "avatar" });
  };
  return (
    <>
      <div className="grid gap-2">
        <Label>{t("routes.qr.logo")}</Label>
        <Select
          value={logoProfileUuid ?? "none"}
          onValueChange={pickLogoProfile}
          disabled={!canEdit}
        >
          <SelectTrigger className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">{t("routes.qr.logo_none")}</SelectItem>
            {profiles
              .slice()
              .sort(
                (a, b) =>
                  Number(b.uuid === suggestedLogoProfileUuid) -
                  Number(a.uuid === suggestedLogoProfileUuid),
              )
              .map((p) => (
                <SelectItem key={p.uuid} value={p.uuid}>
                  {`${p.first_name} ${p.last_name}`.trim() ||
                    p.username ||
                    p.uuid}
                </SelectItem>
              ))}
            {logoProfileUuid &&
            !profiles.some((p) => p.uuid === logoProfileUuid) ? (
              <SelectItem value={logoProfileUuid}>
                {t("routes.qr.logo_profile_unknown")}
              </SelectItem>
            ) : null}
          </SelectContent>
        </Select>
        {logoProfileUuid ? (
          <div className="grid grid-cols-2 gap-2">
            {(PROFILE_COVER_ENABLED || style.logo_field === "cover"
              ? (["avatar", "cover"] as const)
              : (["avatar"] as const)
            ).map((field) => {
              const url = logoMedia[field];
              const active = style.logo_field === field;
              return (
                <button
                  key={field}
                  type="button"
                  aria-pressed={active}
                  disabled={!canEdit || (!url && !logoLoading)}
                  onClick={() => set({ logo_field: field })}
                  className={cn(
                    "flex items-center gap-2 rounded-lg border p-2 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50",
                    active
                      ? "border-primary ring-primary/30 ring-2"
                      : "hover:bg-muted",
                  )}
                >
                  <span className="bg-muted size-10 shrink-0 overflow-hidden rounded-md">
                    {url ? (
                      // eslint-disable-next-line @next/next/no-img-element -- API-owned profile media URL
                      <img
                        src={url}
                        alt=""
                        className="size-full object-cover"
                      />
                    ) : null}
                  </span>
                  <span className="min-w-0">
                    <span className="block font-medium">
                      {t(`routes.qr.logo_${field}`)}
                    </span>
                    {!url && !logoLoading ? (
                      <span className="text-muted-foreground block text-xs">
                        {t("routes.qr.logo_missing")}
                      </span>
                    ) : null}
                  </span>
                </button>
              );
            })}
          </div>
        ) : null}
        {logoMissing ? (
          <p className="text-destructive text-xs">
            {t("routes.qr.logo_pick_media")}
          </p>
        ) : null}
        <p className="text-muted-foreground text-xs">
          {t("routes.qr.logo_hint")}
        </p>
      </div>
      {logoProfileUuid ? (
        <>
          <div className="grid gap-2">
            <div className="flex items-center justify-between">
              <Label htmlFor={`${ids}-logo-size`}>
                {t("routes.qr.logo_size")}
              </Label>
              <span className="text-muted-foreground text-xs tabular-nums">
                {t("routes.qr.logo_size_value", { size: style.logo_size })}
              </span>
            </div>
            <Slider
              id={`${ids}-logo-size`}
              min={LOGO_SIZE_MIN}
              max={LOGO_SIZE_MAX}
              step={1}
              value={[style.logo_size]}
              onValueChange={([v]) => set({ logo_size: v ?? LOGO_SIZE_MIN })}
              disabled={disabled}
            />
          </div>
          <div className="grid gap-1.5">
            <Label>{t("routes.qr.logo_mode")}</Label>
            <ToggleGroup
              type="single"
              variant="outline"
              value={style.logo_mode}
              onValueChange={(v) => v && set({ logo_mode: v })}
              disabled={disabled}
              className="w-full"
            >
              {(["clean", "overlay"] as const).map((m) => (
                <ToggleGroupItem key={m} value={m} className="flex-1">
                  {t(`routes.qr.logo_mode_${m}`)}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
            <p className="text-muted-foreground text-xs">
              {t(`routes.qr.logo_mode_${style.logo_mode}_hint`)}
            </p>
          </div>
        </>
      ) : null}
    </>
  );
}

function SwitchRow({
  id,
  label,
  checked,
  onChange,
  disabled,
}: {
  id: string;
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex items-center gap-2 self-end py-1.5">
      <Switch
        id={id}
        checked={checked}
        onCheckedChange={onChange}
        disabled={disabled}
      />
      <Label htmlFor={id} className="font-normal">
        {label}
      </Label>
    </div>
  );
}

function ColorField({
  id,
  label,
  value,
  onChange,
  disabled,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const valid = isHexColor(value);
  return (
    <div className="grid gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex items-center gap-2">
        <ColorPicker
          aria-label={label}
          value={value}
          onChange={onChange}
          disabled={disabled}
        />
        <Input
          id={id}
          value={value}
          onChange={(e) => {
            const v = e.target.value.trim();
            onChange(v.startsWith("#") ? v : `#${v}`);
          }}
          maxLength={7}
          spellCheck={false}
          autoComplete="off"
          aria-invalid={!valid}
          disabled={disabled}
          className="font-mono uppercase"
        />
      </div>
    </div>
  );
}
