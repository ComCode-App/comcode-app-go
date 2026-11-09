"use client";

import React, { useState } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter, DialogDescription } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Slider } from "@/components/ui/slider";
import { Sparkles, Camera, Orbit, Check } from "lucide-react";
import { Bounded3DStage, Model3DViewboxConfig } from "./Bounded3DStage";
import { toast } from "sonner";
import { productsService } from "../services/products.service";
import { useLocale } from "@/providers/locale-provider";

interface ViewboxStudioModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  productUuid: string;
  productName: string;
  initialViewbox?: Model3DViewboxConfig;
  initialModelUrl?: string;
  initialUsdzUrl?: string;
  onSaved?: (updatedViewbox: Model3DViewboxConfig) => void;
}

export function ViewboxStudioModal({
  open,
  onOpenChange,
  productUuid,
  productName,
  initialViewbox,
  initialModelUrl,
  initialUsdzUrl,
  onSaved,
}: ViewboxStudioModalProps) {
  const { t } = useLocale();
  const [viewbox, setViewbox] = useState<Model3DViewboxConfig>(
    initialViewbox || {
      camera_orbit: { pitch: 18, yaw: 32, distance: 3.2 },
      fov: 45,
      auto_rotate: true,
      stage_bounds: { radius: 2.8, shadow_intensity: 0.45 },
      orbit_limits: { min_pitch: -60, max_pitch: 80, min_zoom: 1.5, max_zoom: 5.5 },
    }
  );

  const [liveCamera, setLiveCamera] = useState({ pitch: 18, yaw: 32, distance: 3.2 });
  const [modelUrl, setModelUrl] = useState(initialModelUrl || "");
  const [usdzUrl, setUsdzUrl] = useState(initialUsdzUrl || "");
  const [isSaving, setIsSaving] = useState(false);
  const [isUploadingGLB, setIsUploadingGLB] = useState(false);
  const [isUploadingUSDZ, setIsUploadingUSDZ] = useState(false);

  const handleCaptureLiveCamera = () => {
    setViewbox((prev) => ({
      ...prev,
      camera_orbit: { ...liveCamera },
    }));
    toast.success(t("products.viewbox.angle_captured"));
  };

  const handleSaveViewbox = async () => {
    try {
      setIsSaving(true);
      await productsService.update3DViewbox(productUuid, {
        model_3d_viewbox: viewbox,
        model_3d_file_url: modelUrl,
        model_3d_usdz_url: usdzUrl,
      });

      toast.success(t("products.viewbox.saved_success"));
      onSaved?.(viewbox);
      onOpenChange(false);
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : t("products.error_updating"));
    } finally {
      setIsSaving(false);
    }
  };

  const handleUpload3DFile = async (e: React.ChangeEvent<HTMLInputElement>, isUSDZ: boolean) => {
    const file = e.target.files?.[0];
    if (!file) return;

    const formData = new FormData();
    formData.append("model", file);

    try {
      if (isUSDZ) setIsUploadingUSDZ(true);
      else setIsUploadingGLB(true);

      const res = await fetch(`/api/v1/platform/products/${productUuid}/3d-model?format=${isUSDZ ? "usdz" : "glb"}`, {
        method: "POST",
        body: formData,
      });

      const data = await res.json();
      if (data.success && data.data) {
        if (isUSDZ) {
          setUsdzUrl(data.data.model_3d_usdz_url);
        } else {
          setModelUrl(data.data.model_3d_file_url);
        }
        toast.success(isUSDZ ? "USDZ (Apple AR) model uploaded" : "GLB 3D Model uploaded");
      } else {
        toast.error(data.error?.message || t("products.error_updating"));
      }
    } catch {
      toast.error(t("common.error_generic"));
    } finally {
      setIsUploadingGLB(false);
      setIsUploadingUSDZ(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl p-0 overflow-hidden bg-background border-border text-foreground">
        <DialogHeader className="p-5 border-b border-border bg-muted/40">
          <div className="flex items-center justify-between">
            <div>
              <DialogTitle className="flex items-center gap-2 text-lg font-semibold">
                <Sparkles className="h-5 w-5 text-amber-500 dark:text-amber-400" />
                {t("products.viewbox.title")}
              </DialogTitle>
              <DialogDescription className="text-xs text-muted-foreground mt-1">
                {t("products.viewbox.description", { productName })}
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="grid grid-cols-1 md:grid-cols-12 gap-0">
          {/* Left: 3D Stage Viewport */}
          <div className="md:col-span-7 p-4 bg-muted/20 dark:bg-zinc-950/60 flex flex-col justify-between">
            <Bounded3DStage
              modelUrl={modelUrl}
              usdzUrl={usdzUrl}
              viewbox={viewbox}
              className="w-full h-80 md:h-[420px] rounded-xl"
              onCameraChange={(orbit) => setLiveCamera(orbit)}
            />

            <div className="mt-3 flex items-center justify-between bg-card border border-border rounded-lg p-2.5 text-xs shadow-xs">
              <div className="flex items-center gap-2 text-foreground font-mono">
                <Camera className="h-3.5 w-3.5 text-primary" />
                <span>Pitch: {liveCamera.pitch}°</span>
                <span>Yaw: {liveCamera.yaw}°</span>
                <span>Dist: {liveCamera.distance}</span>
              </div>
              <Button
                size="sm"
                variant="outline"
                className="h-7 text-xs"
                onClick={handleCaptureLiveCamera}
              >
                <Check className="h-3.5 w-3.5 mr-1 text-emerald-500 dark:text-emerald-400" /> {t("products.viewbox.save_angle_btn")}
              </Button>
            </div>
          </div>

          {/* Right: Controls & Presets */}
          <div className="md:col-span-5 p-4 border-t md:border-t-0 md:border-l border-border bg-card">
            <Tabs defaultValue="camera" className="w-full">
              <TabsList className="grid grid-cols-3 bg-muted h-9 p-0.5">
                <TabsTrigger value="camera" className="text-xs">{t("products.tabs.viewbox")}</TabsTrigger>
                <TabsTrigger value="stage" className="text-xs">{t("products.viewbox.orbit_limits_label")}</TabsTrigger>
                <TabsTrigger value="models" className="text-xs">{t("products.tabs.general")}</TabsTrigger>
              </TabsList>

              {/* Camera Tab */}
              <TabsContent value="camera" className="space-y-4 pt-3">
                <div className="space-y-2">
                  <div className="flex justify-between text-xs">
                    <Label className="text-foreground">{t("products.viewbox.pitch_label")}: {viewbox.camera_orbit?.pitch ?? 18}°</Label>
                  </div>
                  <Slider
                    min={-60}
                    max={80}
                    step={1}
                    value={[viewbox.camera_orbit?.pitch ?? 18]}
                    onValueChange={([val]) =>
                      setViewbox((prev) => ({
                        ...prev,
                        camera_orbit: { ...prev.camera_orbit, pitch: val },
                      }))
                    }
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between text-xs">
                    <Label className="text-foreground">{t("products.viewbox.yaw_label")}: {viewbox.camera_orbit?.yaw ?? 32}°</Label>
                  </div>
                  <Slider
                    min={-180}
                    max={180}
                    step={1}
                    value={[viewbox.camera_orbit?.yaw ?? 32]}
                    onValueChange={([val]) =>
                      setViewbox((prev) => ({
                        ...prev,
                        camera_orbit: { ...prev.camera_orbit, yaw: val },
                      }))
                    }
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between text-xs">
                    <Label className="text-foreground">{t("products.viewbox.distance_label")}: {viewbox.camera_orbit?.distance ?? 3.2}</Label>
                  </div>
                  <Slider
                    min={1.5}
                    max={5.5}
                    step={0.1}
                    value={[viewbox.camera_orbit?.distance ?? 3.2]}
                    onValueChange={([val]) =>
                      setViewbox((prev) => ({
                        ...prev,
                        camera_orbit: { ...prev.camera_orbit, distance: val },
                      }))
                    }
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between text-xs">
                    <Label className="text-foreground">{t("products.viewbox.fov_label")}: {viewbox.fov ?? 45}°</Label>
                  </div>
                  <Slider
                    min={25}
                    max={75}
                    step={1}
                    value={[viewbox.fov ?? 45]}
                    onValueChange={([val]) =>
                      setViewbox((prev) => ({ ...prev, fov: val }))
                    }
                  />
                </div>

                <div className="flex items-center justify-between pt-2 border-t border-border">
                  <Label htmlFor="auto_rotate" className="text-xs text-foreground cursor-pointer">{t("products.viewbox.auto_rotate_label")}</Label>
                  <Switch
                    id="auto_rotate"
                    checked={viewbox.auto_rotate ?? true}
                    onCheckedChange={(checked) =>
                      setViewbox((prev) => ({ ...prev, auto_rotate: checked }))
                    }
                  />
                </div>
              </TabsContent>

              {/* Stage & Limits Tab */}
              <TabsContent value="stage" className="space-y-4 pt-3">
                <div className="space-y-2">
                  <div className="flex justify-between text-xs">
                    <Label className="text-foreground">{t("products.viewbox.stage_radius_label")}: {viewbox.stage_bounds?.radius ?? 2.8}m</Label>
                  </div>
                  <Slider
                    min={1.5}
                    max={5.0}
                    step={0.1}
                    value={[viewbox.stage_bounds?.radius ?? 2.8]}
                    onValueChange={([val]) =>
                      setViewbox((prev) => ({
                        ...prev,
                        stage_bounds: { ...prev.stage_bounds, radius: val },
                      }))
                    }
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between text-xs">
                    <Label className="text-foreground">{t("products.viewbox.shadow_label")}: {Math.round((viewbox.stage_bounds?.shadow_intensity ?? 0.45) * 100)}%</Label>
                  </div>
                  <Slider
                    min={0.1}
                    max={0.9}
                    step={0.05}
                    value={[viewbox.stage_bounds?.shadow_intensity ?? 0.45]}
                    onValueChange={([val]) =>
                      setViewbox((prev) => ({
                        ...prev,
                        stage_bounds: { ...prev.stage_bounds, shadow_intensity: val },
                      }))
                    }
                  />
                </div>

                <div className="space-y-2 pt-2 border-t border-border">
                  <Label className="text-xs text-foreground font-medium flex items-center gap-1.5">
                    <Orbit className="h-3.5 w-3.5 text-primary" /> {t("products.viewbox.orbit_limits_label")}
                  </Label>
                  <div className="grid grid-cols-2 gap-2 text-xs">
                    <div>
                      <span className="text-[10px] text-muted-foreground">Min Zoom</span>
                      <Input
                        type="number"
                        className="h-7 text-xs bg-background border-input"
                        value={viewbox.orbit_limits?.min_zoom ?? 1.5}
                        onChange={(e) => {
                          const v = parseFloat(e.target.value);
                          setViewbox((p) => ({
                            ...p,
                            orbit_limits: { ...p.orbit_limits, min_zoom: v },
                          }));
                        }}
                      />
                    </div>
                    <div>
                      <span className="text-[10px] text-muted-foreground">Max Zoom</span>
                      <Input
                        type="number"
                        className="h-7 text-xs bg-background border-input"
                        value={viewbox.orbit_limits?.max_zoom ?? 5.5}
                        onChange={(e) => {
                          const v = parseFloat(e.target.value);
                          setViewbox((p) => ({
                            ...p,
                            orbit_limits: { ...p.orbit_limits, max_zoom: v },
                          }));
                        }}
                      />
                    </div>
                  </div>
                </div>
              </TabsContent>

              {/* 3D Model Uploads Tab */}
              <TabsContent value="models" className="space-y-4 pt-3">
                <div className="space-y-2">
                  <Label className="text-xs text-foreground font-medium">{t("products.viewbox.glb_label")}</Label>
                  <div className="flex items-center gap-2">
                    <Input
                      type="file"
                      accept=".glb,.gltf"
                      className="text-xs bg-background border-input file:text-xs file:bg-muted file:border-0 file:rounded-md file:text-foreground"
                      onChange={(e) => handleUpload3DFile(e, false)}
                      disabled={isUploadingGLB}
                    />
                  </div>
                  {isUploadingGLB && <p className="text-[11px] text-primary animate-pulse">{t("products.viewbox.glb_uploading")}</p>}
                </div>

                <div className="space-y-2 pt-2 border-t border-border">
                  <Label className="text-xs text-foreground font-medium">{t("products.viewbox.usdz_label")}</Label>
                  <div className="flex items-center gap-2">
                    <Input
                      type="file"
                      accept=".usdz"
                      className="text-xs bg-background border-input file:text-xs file:bg-muted file:border-0 file:rounded-md file:text-foreground"
                      onChange={(e) => handleUpload3DFile(e, true)}
                      disabled={isUploadingUSDZ}
                    />
                  </div>
                  {isUploadingUSDZ && <p className="text-[11px] text-emerald-500 dark:text-emerald-400 animate-pulse">{t("products.viewbox.usdz_uploading")}</p>}
                </div>
              </TabsContent>
            </Tabs>
          </div>
        </div>

        <DialogFooter className="p-4 border-t border-border bg-muted/40 flex justify-end gap-2">
          <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
            {t("common.cancel")}
          </Button>
          <Button
            size="sm"
            onClick={handleSaveViewbox}
            disabled={isSaving}
          >
            {t("products.viewbox.save_settings_btn")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
