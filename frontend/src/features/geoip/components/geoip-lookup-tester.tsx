"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import {
  AlertCircle,
  Building2,
  Clock,
  Compass,
  Globe2,
  Loader2,
  MapPin,
  Network,
  Search,
  Sparkles,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { GeoIPLookupResult } from "@/features/geoip/services/geoip.service";
import { geoipService } from "@/features/geoip/services/geoip.service";
import { isApiError } from "@/lib/api";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const QUICK_SAMPLES = [
  { label: "Google (US)", ip: "8.8.8.8" },
  { label: "Turkey (TR)", ip: "212.252.128.1" },
  { label: "Cloudflare (AU)", ip: "1.1.1.1" },
  { label: "Frankfurt (DE)", ip: "185.199.108.153" },
];

function isPrivateOrLocalIp(ip?: string | null): boolean {
  if (!ip) return false;
  const clean = ip.trim().toLowerCase();
  if (clean === "localhost" || clean === "127.0.0.1" || clean === "::1" || clean === "0.0.0.0") {
    return true;
  }
  if (clean.startsWith("10.") || clean.startsWith("192.168.") || clean.startsWith("169.254.")) {
    return true;
  }
  if (/^172\.(1[6-9]|2[0-9]|3[0-1])\./.test(clean)) {
    return true;
  }
  if (clean.startsWith("fc00:") || clean.startsWith("fd00:") || clean.startsWith("fe80:")) {
    return true;
  }
  return false;
}

export function GeoIPLookupTester() {
  const { t } = useLocale();
  const [ipInput, setIpInput] = useState("");
  const [lookupResult, setLookupResult] = useState<GeoIPLookupResult | null>(null);
  const [isDetectingIp, setIsDetectingIp] = useState(false);

  const lookupMutation = useMutation({
    mutationFn: (targetIp: string) => geoipService.lookup(targetIp),
    onSuccess: (data) => {
      setLookupResult(data);
      if (data.country_code) {
        appToast.success(`${data.ip || "IP"}: ${data.country_name || data.country_code}`);
      } else if (isPrivateOrLocalIp(data.ip)) {
        appToast.info(t("geoip.tester.private_ip_note"));
      } else {
        appToast.info(t("geoip.tester.not_resolved"));
      }
    },
    onError: (error) => {
      if (isApiError(error)) {
        appToast.error(error.message);
      } else {
        appToast.error(t("geoip.error.description"));
      }
    },
  });

  const handleTest = (targetIp?: string) => {
    const ipToTest = targetIp !== undefined ? targetIp : ipInput.trim();
    lookupMutation.mutate(ipToTest);
  };

  const handleUseMyPublicIp = async () => {
    setIsDetectingIp(true);
    let publicIp = "";

    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 3500);

      const res = await fetch("https://api64.ipify.org?format=json", {
        signal: controller.signal,
      });
      clearTimeout(timeoutId);

      if (res.ok) {
        const json = (await res.json()) as { ip?: string };
        if (json?.ip) {
          publicIp = String(json.ip).trim();
        }
      }
    } catch {
      // Fallback if public ipify is blocked / offline
    } finally {
      setIsDetectingIp(false);
    }

    if (publicIp) {
      setIpInput(publicIp);
      handleTest(publicIp);
    } else {
      setIpInput("");
      handleTest("");
    }
  };

  const isPending = lookupMutation.isPending || isDetectingIp;
  const isPrivateIp = isPrivateOrLocalIp(lookupResult?.ip);

  return (
    <Card>
      <CardHeader className="pb-4">
        <div className="flex items-center gap-2">
          <Network className="size-5 text-primary" />
          <CardTitle className="text-lg font-semibold">
            {t("geoip.tester.title")}
          </CardTitle>
        </div>
        <CardDescription>{t("geoip.tester.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-6">
        <div className="space-y-3">
          <Label htmlFor="geoip-test-ip">{t("geoip.tester.input_label")}</Label>
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2.5">
            <div className="relative flex-1">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
              <Input
                id="geoip-test-ip"
                value={ipInput}
                onChange={(e) => setIpInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    handleTest();
                  }
                }}
                placeholder={t("geoip.tester.input_placeholder")}
                className="pl-9"
              />
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                type="button"
                onClick={handleUseMyPublicIp}
                disabled={isPending}
              >
                {isDetectingIp ? (
                  <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                ) : (
                  <Sparkles className="mr-1.5 size-3.5 text-amber-500" />
                )}
                {isDetectingIp ? t("geoip.tester.detecting_ip") : t("geoip.tester.btn_my_ip")}
              </Button>
              <Button
                type="button"
                onClick={() => handleTest()}
                disabled={isPending}
              >
                {lookupMutation.isPending ? (
                  <>
                    <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                    {t("geoip.tester.btn_testing")}
                  </>
                ) : (
                  t("geoip.tester.btn_test")
                )}
              </Button>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-1.5 pt-1">
            <span className="text-xs text-muted-foreground mr-1">
              {t("geoip.tester.quick_samples")}:
            </span>
            {QUICK_SAMPLES.map((sample) => (
              <Button
                key={sample.ip}
                variant="secondary"
                size="sm"
                type="button"
                className="h-6 text-xs px-2.5 font-normal"
                onClick={() => {
                  setIpInput(sample.ip);
                  handleTest(sample.ip);
                }}
              >
                {sample.label}
              </Button>
            ))}
          </div>
        </div>

        {lookupResult ? (
          <div className="rounded-xl border bg-card p-4 shadow-sm space-y-4">
            <div className="flex items-center justify-between border-b pb-3">
              <div className="flex items-center gap-2">
                <Globe2 className="size-4 text-primary" />
                <h4 className="text-sm font-semibold">
                  {t("geoip.tester.result_title")}
                </h4>
              </div>
              <div className="flex items-center gap-2">
                {isPrivateIp ? (
                  <Badge variant="warning">Private / Local Network IP</Badge>
                ) : null}
                <Badge variant={lookupResult.country_code ? "success" : "secondary"}>
                  {lookupResult.ip || "Client IP"}
                </Badge>
              </div>
            </div>

            {isPrivateIp ? (
              <div className="flex items-start gap-2.5 rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-300">
                <AlertCircle className="size-4 shrink-0 mt-0.5" />
                <p>{t("geoip.tester.private_ip_note")}</p>
              </div>
            ) : null}

            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
              <div className="flex items-start gap-2.5 rounded-lg border bg-muted/20 p-3">
                <MapPin className="size-4 text-emerald-600 mt-0.5" />
                <div className="space-y-0.5 overflow-hidden">
                  <p className="text-xs text-muted-foreground font-medium">
                    {t("geoip.tester.country")}
                  </p>
                  <p className="text-sm font-semibold truncate">
                    {lookupResult.country_name || lookupResult.country_code || "N/A"}{" "}
                    {lookupResult.country_code ? `(${lookupResult.country_code})` : ""}
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-2.5 rounded-lg border bg-muted/20 p-3">
                <Building2 className="size-4 text-sky-600 mt-0.5" />
                <div className="space-y-0.5 overflow-hidden">
                  <p className="text-xs text-muted-foreground font-medium">
                    {t("geoip.tester.city")}
                  </p>
                  <p className="text-sm font-semibold truncate">
                    {lookupResult.city || "N/A"}
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-2.5 rounded-lg border bg-muted/20 p-3">
                <Clock className="size-4 text-amber-600 mt-0.5" />
                <div className="space-y-0.5 overflow-hidden">
                  <p className="text-xs text-muted-foreground font-medium">
                    {t("geoip.tester.timezone")}
                  </p>
                  <p className="text-sm font-semibold truncate">
                    {lookupResult.timezone || "N/A"}
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-2.5 rounded-lg border bg-muted/20 p-3">
                <Compass className="size-4 text-indigo-600 mt-0.5" />
                <div className="space-y-0.5 overflow-hidden">
                  <p className="text-xs text-muted-foreground font-medium">
                    {t("geoip.tester.coordinates")}
                  </p>
                  <p className="text-sm font-semibold truncate">
                    {lookupResult.latitude != null && lookupResult.longitude != null
                      ? `${lookupResult.latitude.toFixed(4)}, ${lookupResult.longitude.toFixed(4)}`
                      : "N/A"}
                  </p>
                </div>
              </div>
            </div>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
