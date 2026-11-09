"use client";

import { Loader2 } from "lucide-react";
import { useState } from "react";

import { EntitySectionCard } from "@/components/entity";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useLocale } from "@/providers/locale-provider";

import {
  useCompanySector,
  useSectorCatalog,
  useSectorMutation,
} from "../hooks/use-company-structure";
import { useCompanyStructureAccess } from "../lib/access";
import type { Sector } from "../services/company-structure.service";

const NONE = "__none__";

export function sectorLabel(sector: Sector, locale: string) {
  return locale === "tr"
    ? sector.name_tr || sector.name_en
    : sector.name_en || sector.name_tr;
}

/** Company settings: pick the account's sector from the platform catalog (owner/admin). */
export function CompanySectorCard() {
  const access = useCompanyStructureAccess();
  if (!access.canRead) return null;
  return <CompanySectorCardBody canEdit={access.canManageSector} />;
}

function CompanySectorCardBody({ canEdit }: { canEdit: boolean }) {
  const { t, locale } = useLocale();
  const sectorQuery = useCompanySector();
  const catalogQuery = useSectorCatalog();
  const save = useSectorMutation();
  const [draft, setDraft] = useState<string | null>(null);

  const current = sectorQuery.data?.sector_uuid ?? NONE;
  const value = draft ?? current;
  const dirty = draft !== null && draft !== current;
  const catalog = [...(catalogQuery.data ?? [])].sort((a, b) =>
    sectorLabel(a, locale).localeCompare(sectorLabel(b, locale), locale),
  );

  const onSave = () => {
    save.mutate(
      { sector_uuid: value === NONE ? null : value },
      { onSuccess: () => setDraft(null) },
    );
  };

  return (
    <EntitySectionCard title={t("company_structure.sector.title")}>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
        <label className="grid flex-1 gap-1.5 text-sm">
          <span className="font-medium">
            {t("company_structure.sector.label")}
          </span>
          <Select
            value={value}
            onValueChange={setDraft}
            disabled={
              !canEdit || sectorQuery.isLoading || catalogQuery.isLoading
            }
          >
            <SelectTrigger
              className="w-full sm:max-w-sm"
              aria-label={t("company_structure.sector.label")}
            >
              <SelectValue placeholder={t("common.loading")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>
                {t("company_structure.sector.none")}
              </SelectItem>
              {catalog.map((sector) => (
                <SelectItem key={sector.uuid} value={sector.uuid}>
                  {sectorLabel(sector, locale)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>
        {canEdit ? (
          <Button
            type="button"
            onClick={onSave}
            disabled={!dirty || save.isPending}
          >
            {save.isPending ? (
              <Loader2 className="mr-2 size-4 animate-spin" />
            ) : null}
            {t("common.save")}
          </Button>
        ) : null}
      </div>
      <p className="text-muted-foreground mt-2 text-xs">
        {canEdit
          ? t("company_structure.sector.hint")
          : t("company_structure.sector.read_only")}
      </p>
    </EntitySectionCard>
  );
}
