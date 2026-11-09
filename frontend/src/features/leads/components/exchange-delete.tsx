"use client";

import { CheckCircle2, ShieldX, Trash2 } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { publicLeadsService } from "@/features/leads/services/leads.service";
import { useLocale } from "@/providers/locale-provider";

type State =
  | "idle"
  | "sending"
  | "deleted"
  | "gone"
  | "invalid"
  | "error"
  | "rate_limited";

/** Confirms and performs the visitor's KVKK deletion of a contact exchange (TEC-940). */
export function ExchangeDelete({ token }: { token: string }) {
  const { t } = useLocale();
  const [state, setState] = useState<State>(token ? "idle" : "invalid");

  const confirm = async () => {
    setState("sending");
    try {
      const res = await publicLeadsService.deleteExchange(token);
      if (res.ok) setState("deleted");
      else if (res.status === 404) setState("gone");
      else if (res.status === 400) setState("invalid");
      else if (res.status === 429) setState("rate_limited");
      else setState("error");
    } catch {
      setState("error");
    }
  };

  const done = state === "deleted" || state === "gone";
  const failed = state === "invalid";
  const Icon = done ? CheckCircle2 : failed ? ShieldX : Trash2;
  const body =
    state === "deleted"
      ? "leads.exchange_delete.deleted"
      : state === "gone"
        ? "leads.exchange_delete.gone"
        : state === "invalid"
          ? "leads.exchange_delete.invalid"
          : "leads.exchange_delete.body";

  return (
    <main className="bg-background flex min-h-dvh items-center justify-center px-4">
      <div className="max-w-sm text-center">
        <span className="bg-muted mx-auto flex size-14 items-center justify-center rounded-full">
          <Icon
            className={
              done ? "size-6 text-emerald-600" : "text-muted-foreground size-6"
            }
          />
        </span>
        <h1 className="mt-4 text-xl font-semibold">
          {t("leads.exchange_delete.title")}
        </h1>
        <p className="text-muted-foreground mt-2 text-sm">{t(body)}</p>
        {state === "error" || state === "rate_limited" ? (
          <p className="text-destructive mt-3 text-sm" role="alert">
            {t(
              state === "rate_limited"
                ? "leads.form.error_rate_limited"
                : "leads.exchange_delete.error",
            )}
          </p>
        ) : null}
        {!done && !failed ? (
          <Button
            className="mt-6"
            variant="destructive"
            disabled={state === "sending"}
            onClick={() => void confirm()}
          >
            {t(
              state === "sending"
                ? "leads.exchange_delete.deleting"
                : "leads.exchange_delete.confirm",
            )}
          </Button>
        ) : null}
      </div>
    </main>
  );
}
