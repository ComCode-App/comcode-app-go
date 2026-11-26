"use client";

import type { ColumnDef } from "@tanstack/react-table";
import {
  ArrowLeftRight,
  CreditCard,
  Eye,
  Link2,
  Lock,
  Mail,
  Phone,
  Trash2,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import { createColumn } from "@/components/tables";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { TooltipHint } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

import type { Lead, LeadSource } from "../services/leads.service";

const SOURCE_META: Record<LeadSource, { labelKey: string; icon: LucideIcon }> =
  {
    profile: { labelKey: "leads.source.profile", icon: UserRound },
    card: { labelKey: "leads.source.card", icon: CreditCard },
    smart_link: { labelKey: "leads.source.smart_link", icon: Link2 },
  };

interface LeadsColumnOptions {
  t: (key: string, params?: Record<string, string | number>) => string;
  locale: string;
  canWrite: boolean;
  /** Names of the account's profiles and cards by uuid, to say which one a lead came through. */
  profileNames: Map<string, string>;
  cardNames: Map<string, string>;
  onOpen: (lead: Lead) => void;
  onDelete: (lead: Lead) => void;
}

function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((p) => p[0])
      .join("")
      .toUpperCase() || "?"
  );
}

// Columns for the leads inbox table (TEC-1324): who, how to reach them, where they came
// from (profile, card or smart link and its address), the message and when.
export function getLeadsColumns({
  t,
  locale,
  canWrite,
  profileNames,
  cardNames,
  onOpen,
  onDelete,
}: LeadsColumnOptions): ColumnDef<Lead, unknown>[] {
  const intlLocale = locale === "tr" ? "tr-TR" : "en-GB";

  return [
    createColumn<Lead>({
      accessorKey: "name",
      labelKey: "leads.fields.name",
      enableSorting: false,
      enableHiding: false,
      gridPrimary: true,
      cell: ({ row }) => {
        const lead = row.original;
        const unread = !lead.read_at;
        const subtitle = [lead.job_title, lead.company]
          .filter(Boolean)
          .join(", ");
        return (
          <div className="flex min-w-0 items-center gap-2.5">
            <span
              aria-hidden
              className={cn(
                "size-2 shrink-0 rounded-full",
                unread ? "bg-primary" : "bg-transparent",
              )}
            />
            <span className="bg-primary/10 text-primary flex size-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold">
              {lead.locked ? (
                <Lock className="size-3.5" />
              ) : (
                initials(lead.name)
              )}
            </span>
            <button
              type="button"
              onClick={() => onOpen(lead)}
              className="min-w-0 text-left"
            >
              <span className="flex items-center gap-1.5">
                <span
                  className={cn(
                    "truncate text-sm hover:underline",
                    unread && "font-semibold",
                  )}
                >
                  {lead.name}
                </span>
                {unread ? (
                  <Badge className="px-1.5 py-0 text-[10px]">
                    {t("leads.status.new")}
                  </Badge>
                ) : null}
                {lead.kind === "exchange" ? (
                  <Badge
                    variant="outline"
                    className="gap-1 px-1.5 py-0 text-[10px]"
                  >
                    <ArrowLeftRight className="size-3" />
                    {t("leads.kind.exchange")}
                  </Badge>
                ) : null}
                {lead.locked ? (
                  <Badge
                    variant="secondary"
                    className="gap-1 px-1.5 py-0 text-[10px]"
                  >
                    <Lock className="size-3" />
                    {t("leads.locked.badge")}
                  </Badge>
                ) : null}
              </span>
              {subtitle ? (
                <span className="text-muted-foreground block truncate text-xs">
                  {subtitle}
                </span>
              ) : null}
            </button>
          </div>
        );
      },
    }),
    createColumn<Lead>({
      id: "contact",
      labelKey: "leads.columns.contact",
      enableSorting: false,
      cell: ({ row }) => {
        const lead = row.original;
        if (!lead.email && !lead.phone) {
          return <span className="text-muted-foreground text-xs">—</span>;
        }
        return (
          <div className="flex min-w-0 flex-col gap-0.5 text-xs">
            {lead.email ? (
              <span className="flex min-w-0 items-center gap-1.5">
                <Mail className="text-muted-foreground size-3.5 shrink-0" />
                <span className="truncate">{lead.email}</span>
              </span>
            ) : null}
            {lead.phone ? (
              <span className="flex min-w-0 items-center gap-1.5">
                <Phone className="text-muted-foreground size-3.5 shrink-0" />
                <span className="truncate">{lead.phone}</span>
              </span>
            ) : null}
          </div>
        );
      },
    }),
    createColumn<Lead>({
      accessorKey: "source_type",
      labelKey: "leads.fields.source_type",
      enableSorting: false,
      gridSecondary: true,
      cell: ({ row }) => {
        const lead = row.original;
        const meta = SOURCE_META[lead.source_type];
        const Icon = meta?.icon ?? Link2;
        const name =
          (lead.card_uuid && cardNames.get(lead.card_uuid)) ||
          (lead.source_type === "profile" &&
            lead.profile_uuid &&
            profileNames.get(lead.profile_uuid)) ||
          "";
        return (
          <div className="flex min-w-0 items-center gap-2">
            <span className="bg-muted text-muted-foreground flex size-7 shrink-0 items-center justify-center rounded-md">
              <Icon className="size-3.5" />
            </span>
            <div className="flex min-w-0 flex-col text-xs">
              <span className="text-foreground truncate font-medium">
                {meta ? t(meta.labelKey) : lead.source_type}
                {name ? ` · ${name}` : ""}
              </span>
              {lead.route_path ? (
                <a
                  href={`/${lead.route_path}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  onClick={(e) => e.stopPropagation()}
                  className="text-primary truncate font-mono hover:underline"
                >
                  /{lead.route_path}
                </a>
              ) : null}
            </div>
          </div>
        );
      },
    }),
    createColumn<Lead>({
      accessorKey: "message",
      labelKey: "leads.fields.message",
      enableSorting: false,
      cell: ({ row }) =>
        row.original.message ? (
          <span className="text-muted-foreground line-clamp-2 max-w-xs text-xs">
            {row.original.message}
          </span>
        ) : (
          <span className="text-muted-foreground text-xs">—</span>
        ),
    }),
    createColumn<Lead>({
      accessorKey: "created_at",
      labelKey: "leads.fields.created_at",
      enableSorting: false,
      cell: ({ row }) => {
        const d = new Date(row.original.created_at);
        return (
          <div className="flex flex-col text-xs whitespace-nowrap">
            <span className="text-foreground">
              {d.toLocaleDateString(intlLocale, { dateStyle: "medium" })}
            </span>
            <span className="text-muted-foreground">
              {d.toLocaleTimeString(intlLocale, { timeStyle: "short" })}
            </span>
          </div>
        );
      },
    }),
    createColumn<Lead>({
      id: "actions",
      labelKey: "table.actions",
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const lead = row.original;
        return (
          <div
            className="flex items-center justify-end gap-1"
            onClick={(e) => e.stopPropagation()}
          >
            <TooltipHint label={t("leads.actions.open")}>
              <Button
                variant="ghost"
                size="icon"
                className="text-muted-foreground hover:text-foreground h-8 w-8"
                onClick={() => onOpen(lead)}
                aria-label={t("leads.actions.open")}
              >
                <Eye className="size-4" />
              </Button>
            </TooltipHint>
            {canWrite ? (
              <TooltipHint label={t("leads.actions.delete")}>
                <Button
                  variant="ghost"
                  size="icon"
                  className="text-muted-foreground hover:text-destructive h-8 w-8"
                  onClick={() => onDelete(lead)}
                  aria-label={t("leads.actions.delete")}
                >
                  <Trash2 className="size-4" />
                </Button>
              </TooltipHint>
            ) : null}
          </div>
        );
      },
    }),
  ];
}
