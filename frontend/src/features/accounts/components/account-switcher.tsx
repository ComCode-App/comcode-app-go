"use client";

import { Building2, Check, ChevronsUpDown, User } from "lucide-react";
import { useState } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar";
import { routes } from "@/config/routes";
import type { AccountSummary } from "@/lib/auth/types";
import { useAuth } from "@/providers/auth-provider";
import { useLocale } from "@/providers/locale-provider";
import { appToast } from "@/providers/toast-provider";
import { authService } from "@/services/auth.service";

function AccountIcon({ account }: { account?: AccountSummary }) {
  const Icon = account?.type === "company" ? Building2 : User;
  return (
    <span className="bg-muted text-muted-foreground flex size-8 shrink-0 items-center justify-center rounded-md">
      <Icon className="size-4" />
    </span>
  );
}

/**
 * Switches the session's active account. App data is scoped to the active account, so the page
 * reloads after a switch to drop everything cached for the previous one.
 */
export function AccountSwitcher() {
  const { user, hydrateProfile } = useAuth();
  const { t } = useLocale();
  const [switching, setSwitching] = useState(false);

  const active = user?.account;
  const accounts = user?.accounts ?? [];
  if (!active) return null;

  const switchTo = async (account: AccountSummary) => {
    if (account.uuid === active.uuid || switching) return;
    setSwitching(true);
    try {
      await authService.switchAccount(account.uuid);
      await hydrateProfile();
      window.location.assign(routes.app.home);
    } catch {
      appToast.error(t("accounts.switch_failed"));
      setSwitching(false);
    }
  };

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              disabled={switching}
              tooltip={active.name}
              aria-label={t("accounts.switch")}
            >
              <AccountIcon account={active} />
              <span className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{active.name}</span>
                <span className="text-muted-foreground truncate text-xs">
                  {t(`accounts.types.${active.type}`)} · {t(`accounts.roles.${active.role}`)}
                </span>
              </span>
              <ChevronsUpDown className="ml-auto size-4" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="min-w-64">
            <DropdownMenuLabel>{t("accounts.switch")}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {accounts.map((account) => (
              <DropdownMenuItem
                key={account.uuid}
                disabled={account.status !== "active"}
                onSelect={() => void switchTo(account)}
                className="gap-2"
              >
                <AccountIcon account={account} />
                <span className="grid flex-1 leading-tight">
                  <span className="truncate">{account.name}</span>
                  <span className="text-muted-foreground text-xs">
                    {t(`accounts.roles.${account.role}`)}
                  </span>
                </span>
                {account.uuid === active.uuid ? <Check className="size-4" /> : null}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
