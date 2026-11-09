"use client";

import React, { useEffect, useRef, useState } from "react";
import * as THREE from "three";
import { OrbitControls } from "three/examples/jsm/controls/OrbitControls.js";
import { GLTFLoader } from "three/examples/jsm/loaders/GLTFLoader.js";
import { Maximize2, RotateCw, Sparkles, Smartphone, Eye } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useTheme } from "next-themes";
import { useLocale } from "@/providers/locale-provider";

export interface CameraOrbitConfig {
  pitch?: number; // deg (polar angle)
  yaw?: number; // deg (azimuth angle)
  distance?: number;
}

export interface StageBoundsConfig {
  radius?: number;
  height?: number;
  shadow_intensity?: number;
}

export interface OrbitLimitsConfig {
  min_pitch?: number;
  max_pitch?: number;
  min_yaw?: number;
  max_yaw?: number;
  min_zoom?: number;
  max_zoom?: number;
}

export interface Model3DViewboxConfig {
  camera_orbit?: CameraOrbitConfig;
  fov?: number;
  auto_rotate?: boolean;
  rotation_speed?: number;
  stage_bounds?: StageBoundsConfig;
  orbit_limits?: OrbitLimitsConfig;
  environment_preset?: string;
  background_color?: string;
}

interface Bounded3DStageProps {
  modelUrl?: string;
  usdzUrl?: string;
  viewbox?: Model3DViewboxConfig;
  interactive?: boolean;
  autoRotateDefault?: boolean;
  cardColor?: string;
  cardMaterial?: string;
  className?: string;
  onCameraChange?: (
    orbit: { pitch: number; yaw: number; distance: number },
    fov: number,
  ) => void;
}

export function Bounded3DStage({
  modelUrl,
  usdzUrl,
  viewbox,
  interactive = true,
  autoRotateDefault,
  cardColor = "#18181b",
  cardMaterial = "metal_black",
  className = "w-full h-80 md:h-96",
  onCameraChange,
}: Bounded3DStageProps) {
  const { t } = useLocale();
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme === "dark";

  const containerRef = useRef<HTMLDivElement>(null);
  const [isRotating, setIsRotating] = useState(
    autoRotateDefault ?? viewbox?.auto_rotate ?? true,
  );
  const [isLoading, setIsLoading] = useState(false);
  const [supportsAR] = useState(() => {
    if (typeof window === "undefined") return false;
    const a = document.createElement("a");
    return Boolean(a.relList && a.relList.supports && a.relList.supports("ar"));
  });

  const controlsRef = useRef<OrbitControls | null>(null);
  const cameraRef = useRef<THREE.PerspectiveCamera | null>(null);
  const reqIdRef = useRef<number | null>(null);
  const onCameraChangeRef = useRef(onCameraChange);

  useEffect(() => {
    onCameraChangeRef.current = onCameraChange;
  });

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const width = container.clientWidth || 400;
    const height = container.clientHeight || 350;

    // 1. Scene
    const scene = new THREE.Scene();
    const defaultBg = isDark ? "#09090b" : "#f8fafc";
    scene.background = new THREE.Color(viewbox?.background_color || defaultBg);

    // 2. Camera
    const fov = viewbox?.fov || 45;
    const camera = new THREE.PerspectiveCamera(fov, width / height, 0.1, 100);
    cameraRef.current = camera;

    const pitchRad = THREE.MathUtils.degToRad(
      viewbox?.camera_orbit?.pitch ?? 20,
    );
    const yawRad = THREE.MathUtils.degToRad(viewbox?.camera_orbit?.yaw ?? 30);
    const distance = viewbox?.camera_orbit?.distance ?? 3.2;

    camera.position.x = distance * Math.sin(yawRad) * Math.cos(pitchRad);
    camera.position.y = distance * Math.sin(pitchRad);
    camera.position.z = distance * Math.cos(yawRad) * Math.cos(pitchRad);
    camera.lookAt(0, 0, 0);

    // 3. Renderer
    const renderer = new THREE.WebGLRenderer({
      antialias: true,
      alpha: true,
      powerPreference: "high-performance",
    });
    renderer.setSize(width, height);
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.shadowMap.enabled = true;
    renderer.shadowMap.type = THREE.PCFShadowMap;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    renderer.toneMappingExposure = isDark ? 1.1 : 1.0;

    container.replaceChildren(renderer.domElement);

    // 4. Controls
    const controls = new OrbitControls(camera, renderer.domElement);
    controlsRef.current = controls;
    controls.enableDamping = true;
    controls.dampingFactor = 0.05;
    controls.enabled = interactive;
    controls.autoRotate = isRotating;
    controls.autoRotateSpeed = viewbox?.rotation_speed
      ? viewbox.rotation_speed * 1.5
      : 1.5;

    // Apply Orbit Limits
    if (viewbox?.orbit_limits) {
      if (viewbox.orbit_limits.max_pitch !== undefined) {
        controls.minPolarAngle = THREE.MathUtils.degToRad(
          90 - viewbox.orbit_limits.max_pitch,
        );
      }
      if (viewbox.orbit_limits.min_pitch !== undefined) {
        controls.maxPolarAngle = THREE.MathUtils.degToRad(
          90 - viewbox.orbit_limits.min_pitch,
        );
      }
      if (viewbox.orbit_limits.min_zoom !== undefined) {
        controls.minDistance = viewbox.orbit_limits.min_zoom;
      } else {
        controls.minDistance = 1.2;
      }
      if (viewbox.orbit_limits.max_zoom !== undefined) {
        controls.maxDistance = viewbox.orbit_limits.max_zoom;
      } else {
        controls.maxDistance = 6.0;
      }
    } else {
      controls.minDistance = 1.2;
      controls.maxDistance = 6.0;
      controls.minPolarAngle = THREE.MathUtils.degToRad(15);
      controls.maxPolarAngle = THREE.MathUtils.degToRad(85);
    }

    // 5. Lights
    const ambientIntensity = isDark ? 0.8 : 1.1;
    const ambientLight = new THREE.AmbientLight(0xffffff, ambientIntensity);
    scene.add(ambientLight);

    const dirLight = new THREE.DirectionalLight(0xffffff, isDark ? 1.8 : 1.5);
    dirLight.position.set(4, 6, 4);
    dirLight.castShadow = true;
    dirLight.shadow.mapSize.width = 1024;
    dirLight.shadow.mapSize.height = 1024;
    dirLight.shadow.bias = -0.0001;
    scene.add(dirLight);

    const rimLightColor = isDark ? 0x60a5fa : 0x3b82f6;
    const rimLight = new THREE.DirectionalLight(
      rimLightColor,
      isDark ? 1.2 : 0.7,
    );
    rimLight.position.set(-4, 3, -4);
    scene.add(rimLight);

    const goldFill = new THREE.PointLight(0xf59e0b, isDark ? 0.6 : 0.4, 10);
    goldFill.position.set(0, -1, 3);
    scene.add(goldFill);

    // 6. Bounded Stage Platform & Contact Shadow
    const stageRadius = viewbox?.stage_bounds?.radius ?? 2.8;
    const shadowIntensity =
      viewbox?.stage_bounds?.shadow_intensity ?? (isDark ? 0.45 : 0.25);

    // Bounded Stage Disc
    const stageGeo = new THREE.CylinderGeometry(
      stageRadius,
      stageRadius + 0.1,
      0.04,
      64,
    );
    const stageMat = new THREE.MeshStandardMaterial({
      color: isDark ? 0x18181b : 0xe2e8f0,
      roughness: isDark ? 0.8 : 0.7,
      metalness: 0.1,
    });
    const stage = new THREE.Mesh(stageGeo, stageMat);
    stage.position.y = -0.65;
    stage.receiveShadow = true;
    scene.add(stage);

    // Stage boundary accent ring
    const ringGeo = new THREE.RingGeometry(stageRadius - 0.04, stageRadius, 64);
    const ringMat = new THREE.MeshBasicMaterial({
      color: isDark ? 0x3f3f46 : 0xcbd5e1,
      side: THREE.DoubleSide,
    });
    const ring = new THREE.Mesh(ringGeo, ringMat);
    ring.rotation.x = -Math.PI / 2;
    ring.position.y = -0.629;
    scene.add(ring);

    // Soft Contact Shadow Plane
    const shadowGeo = new THREE.PlaneGeometry(
      stageRadius * 1.8,
      stageRadius * 1.8,
    );
    // Radial shadow gradient canvas texture
    const canvas = document.createElement("canvas");
    canvas.width = 256;
    canvas.height = 256;
    const ctx = canvas.getContext("2d");
    if (ctx) {
      const grad = ctx.createRadialGradient(128, 128, 10, 128, 128, 120);
      grad.addColorStop(0, `rgba(0,0,0,${shadowIntensity})`);
      grad.addColorStop(0.4, `rgba(0,0,0,${shadowIntensity * 0.5})`);
      grad.addColorStop(1, "rgba(0,0,0,0)");
      ctx.fillStyle = grad;
      ctx.fillRect(0, 0, 256, 256);
    }
    const shadowTex = new THREE.CanvasTexture(canvas);
    const shadowMat = new THREE.MeshBasicMaterial({
      map: shadowTex,
      transparent: true,
      depthWrite: false,
    });
    const shadowPlane = new THREE.Mesh(shadowGeo, shadowMat);
    shadowPlane.rotation.x = -Math.PI / 2;
    shadowPlane.position.y = -0.628;
    scene.add(shadowPlane);

    // 7. 3D Model: GLTF/GLB or Procedural Premium NFC Smart Card
    const modelGroup = new THREE.Group();
    scene.add(modelGroup);

    if (modelUrl) {
      queueMicrotask(() => setIsLoading(true));
      const loader = new GLTFLoader();
      loader.load(
        modelUrl,
        (gltf) => {
          setIsLoading(false);
          const root = gltf.scene;
          root.traverse((child) => {
            if ((child as THREE.Mesh).isMesh) {
              child.castShadow = true;
              child.receiveShadow = true;
            }
          });

          // Center and scale model appropriately
          const box = new THREE.Box3().setFromObject(root);
          const size = box.getSize(new THREE.Vector3());
          const maxDim = Math.max(size.x, size.y, size.z);
          if (maxDim > 0) {
            const scale = 1.6 / maxDim;
            root.scale.set(scale, scale, scale);
          }
          const center = box.getCenter(new THREE.Vector3());
          root.position.sub(center.multiplyScalar(root.scale.x));
          root.position.y = -0.05;

          modelGroup.add(root);
        },
        undefined,
        (err) => {
          setIsLoading(false);
          console.warn(
            "Could not load 3D GLB model, falling back to card mesh",
            err,
          );
          createProceduralCard(modelGroup, cardColor, cardMaterial);
        },
      );
    } else {
      createProceduralCard(modelGroup, cardColor, cardMaterial);
    }

    // 8. Event Listener for Viewbox updates
    controls.addEventListener("change", () => {
      if (onCameraChangeRef.current) {
        const camPos = camera.position;
        const dist = camPos.length();
        const pitch = THREE.MathUtils.radToDeg(Math.asin(camPos.y / dist));
        const yaw = THREE.MathUtils.radToDeg(Math.atan2(camPos.x, camPos.z));
        onCameraChangeRef.current(
          {
            pitch: Math.round(pitch),
            yaw: Math.round(yaw),
            distance: parseFloat(dist.toFixed(2)),
          },
          camera.fov,
        );
      }
    });

    // 9. Resize observer
    const handleResize = () => {
      if (!container) return;
      const w = container.clientWidth;
      const h = container.clientHeight;
      camera.aspect = w / h;
      camera.updateProjectionMatrix();
      renderer.setSize(w, h);
    };

    const resizeObserver = new ResizeObserver(handleResize);
    resizeObserver.observe(container);

    // 10. Animation Loop
    let animId = 0;
    const animate = () => {
      animId = requestAnimationFrame(animate);
      controls.update();
      renderer.render(scene, camera);
    };
    animate();
    reqIdRef.current = animId;

    return () => {
      cancelAnimationFrame(animId);
      resizeObserver.disconnect();
      renderer.dispose();
      stageGeo.dispose();
      stageMat.dispose();
      ringGeo.dispose();
      ringMat.dispose();
      shadowGeo.dispose();
      shadowMat.dispose();
      shadowTex.dispose();
    };
  }, [
    modelUrl,
    viewbox,
    interactive,
    cardColor,
    cardMaterial,
    isRotating,
    isDark,
  ]);

  // Update rotation dynamically
  useEffect(() => {
    if (controlsRef.current) {
      controlsRef.current.autoRotate = isRotating;
    }
  }, [isRotating]);

  const resetView = () => {
    if (!controlsRef.current || !cameraRef.current) return;
    const pitchRad = THREE.MathUtils.degToRad(
      viewbox?.camera_orbit?.pitch ?? 20,
    );
    const yawRad = THREE.MathUtils.degToRad(viewbox?.camera_orbit?.yaw ?? 30);
    const distance = viewbox?.camera_orbit?.distance ?? 3.2;

    cameraRef.current.position.x =
      distance * Math.sin(yawRad) * Math.cos(pitchRad);
    cameraRef.current.position.y = distance * Math.sin(pitchRad);
    cameraRef.current.position.z =
      distance * Math.cos(yawRad) * Math.cos(pitchRad);
    cameraRef.current.lookAt(0, 0, 0);
    controlsRef.current.reset();
  };

  return (
    <div
      className={`border-border/80 bg-muted/30 relative overflow-hidden rounded-2xl border shadow-sm dark:bg-zinc-950/90 ${className}`}
    >
      {/* 3D Canvas Container */}
      <div
        ref={containerRef}
        className="h-full w-full cursor-grab active:cursor-grabbing"
      />

      {/* Loading Overlay */}
      {isLoading && (
        <div className="bg-background/70 absolute inset-0 z-10 flex items-center justify-center backdrop-blur-xs">
          <div className="flex flex-col items-center gap-2">
            <div className="border-primary h-8 w-8 animate-spin rounded-full border-2 border-t-transparent" />
            <span className="text-muted-foreground text-xs font-medium">
              {t("products.stage.loading")}
            </span>
          </div>
        </div>
      )}

      {/* Controls Overlay Header */}
      <div className="pointer-events-none absolute top-3 right-3 left-3 z-10 flex items-center justify-between">
        <div className="pointer-events-auto flex items-center gap-1.5">
          <Badge
            variant="outline"
            className="bg-background/85 border-border/80 text-foreground px-2 py-0.5 font-mono text-xs shadow-xs backdrop-blur-md dark:bg-zinc-900/80"
          >
            <Sparkles className="mr-1 h-3 w-3 text-amber-500 dark:text-amber-400" />
            3D Studio
          </Badge>
          {usdzUrl && (
            <Badge
              variant="outline"
              className="bg-background/85 border-border/80 text-[11px] text-emerald-600 shadow-xs backdrop-blur-md dark:bg-zinc-900/80 dark:text-emerald-400"
            >
              USDZ / AR
            </Badge>
          )}
        </div>

        <div className="pointer-events-auto flex items-center gap-1.5">
          <Button
            size="icon"
            variant="ghost"
            className="bg-background/85 border-border/80 hover:bg-muted text-foreground h-8 w-8 rounded-lg border shadow-xs backdrop-blur-md dark:bg-zinc-900/80"
            onClick={() => setIsRotating(!isRotating)}
            title={
              isRotating
                ? t("products.stage.stop_rotation")
                : t("products.stage.start_rotation")
            }
          >
            <RotateCw
              className={`h-4 w-4 ${isRotating ? "text-primary animate-spin" : ""}`}
              style={{ animationDuration: "6s" }}
            />
          </Button>

          <Button
            size="icon"
            variant="ghost"
            className="bg-background/85 border-border/80 hover:bg-muted text-foreground h-8 w-8 rounded-lg border shadow-xs backdrop-blur-md dark:bg-zinc-900/80"
            onClick={resetView}
            title={t("products.stage.reset_camera")}
          >
            <Maximize2 className="h-4 w-4" />
          </Button>

          {usdzUrl && supportsAR && (
            <a
              href={usdzUrl}
              rel="ar"
              className="bg-primary text-primary-foreground inline-flex h-8 items-center gap-1 rounded-lg px-2.5 text-xs font-medium shadow-xs transition-opacity hover:opacity-90"
              title={t("products.stage.ar_quicklook")}
            >
              <Smartphone className="h-3.5 w-3.5" />
              <span>AR</span>
            </a>
          )}
        </div>
      </div>

      {/* Stage Hint Footer */}
      <div className="text-muted-foreground pointer-events-none absolute right-3 bottom-2.5 left-3 flex items-center justify-between text-[11px]">
        <span className="bg-background/80 border-border/60 flex items-center gap-1 rounded-md border px-2 py-0.5 shadow-xs backdrop-blur-xs dark:bg-zinc-900/60">
          <Eye className="h-3 w-3" /> {t("products.stage.drag_to_rotate")}
        </span>
        <span className="bg-background/80 border-border/60 rounded border px-1.5 py-0.5 font-mono text-[10px] shadow-xs backdrop-blur-xs dark:bg-zinc-900/60">
          Turntable Stage
        </span>
      </div>
    </div>
  );
}

// Helper to create high quality procedural NFC Card
function createProceduralCard(
  group: THREE.Group,
  colorHex: string,
  materialType: string,
) {
  group.clear();

  // Standard Credit Card aspect ratio (85.6mm x 53.98mm x 0.8mm) => ~1.6 x 1.0 x 0.04
  const cardWidth = 1.7;
  const cardHeight = 1.08;
  const cardDepth = 0.035;
  const radius = 0.08;

  // Rounded rectangle shape
  const shape = new THREE.Shape();
  const x = -cardWidth / 2;
  const y = -cardHeight / 2;
  shape.moveTo(x + radius, y);
  shape.lineTo(x + cardWidth - radius, y);
  shape.quadraticCurveTo(x + cardWidth, y, x + cardWidth, y + radius);
  shape.lineTo(x + cardWidth, y + cardHeight - radius);
  shape.quadraticCurveTo(
    x + cardWidth,
    y + cardHeight,
    x + cardWidth - radius,
    y + cardHeight,
  );
  shape.lineTo(x + radius, y + cardHeight);
  shape.quadraticCurveTo(x, y + cardHeight, x, y + cardHeight - radius);
  shape.lineTo(x, y + radius);
  shape.quadraticCurveTo(x, y, x + radius, y);

  const extrudeSettings = {
    steps: 1,
    depth: cardDepth,
    bevelEnabled: true,
    bevelThickness: 0.01,
    bevelSize: 0.01,
    bevelOffset: 0,
    bevelSegments: 4,
  };

  const cardGeo = new THREE.ExtrudeGeometry(shape, extrudeSettings);
  cardGeo.center();

  let metalness = 0.85;
  let roughness = 0.25;
  let color = new THREE.Color(colorHex);

  if (materialType.includes("gold")) {
    color = new THREE.Color("#eab308");
    metalness = 0.95;
    roughness = 0.18;
  } else if (materialType.includes("silver")) {
    color = new THREE.Color("#cbd5e1");
    metalness = 0.95;
    roughness = 0.2;
  } else if (materialType.includes("wood")) {
    color = new THREE.Color("#78350f");
    metalness = 0.05;
    roughness = 0.65;
  } else if (materialType.includes("pvc")) {
    metalness = 0.15;
    roughness = 0.35;
  }

  const cardMat = new THREE.MeshStandardMaterial({
    color: color,
    metalness: metalness,
    roughness: roughness,
  });

  const cardMesh = new THREE.Mesh(cardGeo, cardMat);
  cardMesh.castShadow = true;
  cardMesh.receiveShadow = true;
  cardMesh.rotation.x = THREE.MathUtils.degToRad(-10);
  group.add(cardMesh);

  // NFC Chip icon plate on the card
  const chipGeo = new THREE.BoxGeometry(0.25, 0.2, 0.005);
  const chipMat = new THREE.MeshStandardMaterial({
    color: 0xd97706,
    metalness: 0.9,
    roughness: 0.3,
  });
  const chipMesh = new THREE.Mesh(chipGeo, chipMat);
  chipMesh.position.set(-0.55, 0.05, cardDepth / 2 + 0.015);
  chipMesh.rotation.x = THREE.MathUtils.degToRad(-10);
  group.add(chipMesh);

  // Logo Laser Etch accent
  const logoGeo = new THREE.PlaneGeometry(0.4, 0.15);
  const logoMat = new THREE.MeshStandardMaterial({
    color: materialType.includes("gold") ? 0xffffff : 0xe2e8f0,
    metalness: 0.9,
    roughness: 0.1,
  });
  const logoMesh = new THREE.Mesh(logoGeo, logoMat);
  logoMesh.position.set(0.45, -0.3, cardDepth / 2 + 0.015);
  logoMesh.rotation.x = THREE.MathUtils.degToRad(-10);
  group.add(logoMesh);
}
