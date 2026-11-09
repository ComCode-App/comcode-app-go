"use client";

import { FileDown } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { routes } from "@/config/routes";
import { exportsService } from "@/features/io/services/exports.service";
import type { ExportFormat } from "@/features/io/types";
import {
  platformDownloadFile,
  triggerBrowserDownload,
} from "@/lib/api/platform-form-request";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";

const FORMATS: ExportFormat[] = ["xlsx", "csv", "pdf"];

/**
 * Production hand-off for the factory (TEC-581): the manifest export (serials, /c/{code} payloads,
 * activation codes; step-up protected) and a ZIP of QR images for printing.
 */
export function OrderManifestMenu({
  orderUuid,
  disabled,
}: {
  orderUuid: string;
  disabled?: boolean;
}) {
  const { t, locale } = useLocale();
  const [pending, setPending] = useState(false);

  const run = async (fn: () => Promise<void>) => {
    setPending(true);
    try {
      await fn();
    } finally {
      setPending(false);
    }
  };

  const exportManifest = (format: ExportFormat) =>
    run(async () => {
      try {
        await exportsService.request(
          `/v1/platform/orders/${orderUuid}/manifest/export`,
          { format, locale },
        );
        appToast.success(t("exports.toast.queued"));
      } catch {
        appToast.error(t("exports.toast.failed"));
      }
    });

  const downloadQRCodes = () =>
    run(async () => {
      try {
        const { blob, filename } = await platformDownloadFile(
          `/v1/platform/orders/${orderUuid}/manifest/qr-codes`,
        );
        triggerBrowserDownload(blob, filename ?? "qr-codes.zip");
      } catch {
        appToast.error(t("orders.manifest.qr_failed"));
      }
    });

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          size="sm"
          variant="outline"
          className="text-xs"
          disabled={disabled || pending}
        >
          <FileDown className="mr-1.5 h-3.5 w-3.5" />
          {t("orders.manifest.menu")}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel className="text-xs">
          {t("orders.manifest.export_label")}
        </DropdownMenuLabel>
        {FORMATS.map((format) => (
          <DropdownMenuItem
            key={format}
            onClick={() => void exportManifest(format)}
          >
            {t(`exports.formats.${format}`)}
          </DropdownMenuItem>
        ))}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => void downloadQRCodes()}>
          {t("orders.manifest.qr_zip")}
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href={routes.platform.exports.root}>
            {t("exports.view_jobs")}
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
