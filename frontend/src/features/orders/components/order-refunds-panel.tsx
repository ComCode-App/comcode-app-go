"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { AlertTriangle, RotateCcw, Undo2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useLocale } from "@/providers/locale-provider";
import { useDialogs } from "@/providers/dialog-provider";
import { ordersService, type OrderRefund, type OrderRefundablePayment } from "../services/orders.service";
import { ordersQueryKeys } from "../hooks/query-keys";

interface OrderRefundsPanelProps {
  orderUuid: string;
}

/**
 * Provider refunds of an order's online payments (TEC-935): what was paid and refunded, a full
 * or partial refund through iyzico/PayTR, and failed refunds with the provider error and a retry.
 */
export function OrderRefundsPanel({ orderUuid }: OrderRefundsPanelProps) {
  const { t, locale } = useLocale();
  const { confirm } = useDialogs();
  const queryClient = useQueryClient();
  const [amount, setAmount] = useState("");
  const [reason, setReason] = useState("");

  const { data } = useQuery({
    queryKey: ordersQueryKeys.refunds(orderUuid),
    queryFn: () => ordersService.listRefunds(orderUuid),
  });

  const money = (minor: number, currency: string) =>
    `${(minor / 100).toLocaleString(locale === "tr" ? "tr-TR" : "en-US", { minimumFractionDigits: 2 })} ${currency}`;

  const refresh = () => queryClient.invalidateQueries({ queryKey: ordersQueryKeys.detail(orderUuid) });
  const onError = (err: Error) => {
    toast.error(err?.message || t("orders.refund.error"));
    refresh();
  };

  const refundMutation = useMutation({
    mutationFn: (vars: { payment: OrderRefundablePayment; amountMinor?: number; key: string }) =>
      ordersService.refund(orderUuid, {
        payment_uuid: vars.payment.uuid,
        amount_minor: vars.amountMinor,
        reason: reason.trim() || undefined,
        idempotency_key: vars.key,
      }),
    onSuccess: () => {
      toast.success(t("orders.refund.success"));
      setAmount("");
      setReason("");
      refresh();
    },
    onError,
  });

  const retryMutation = useMutation({
    mutationFn: (refund: OrderRefund) => ordersService.retryRefund(orderUuid, refund.uuid),
    onSuccess: () => {
      toast.success(t("orders.refund.success"));
      refresh();
    },
    onError,
  });

  if (!data || data.payments.length === 0) {
    return <p className="text-xs text-muted-foreground">{t("orders.refund.no_online_payment")}</p>;
  }

  const startRefund = async (payment: OrderRefundablePayment) => {
    let amountMinor: number | undefined;
    if (amount.trim() !== "") {
      amountMinor = Math.round(Number(amount.replace(",", ".")) * 100);
      if (!Number.isFinite(amountMinor) || amountMinor <= 0 || amountMinor > payment.refundable_minor) {
        toast.error(t("orders.refund.invalid_amount"));
        return;
      }
    }
    const value = money(amountMinor ?? payment.refundable_minor, payment.currency);
    const full = amountMinor === undefined || amountMinor === payment.refundable_minor;
    const ok = await confirm({
      title: t("orders.refund.confirm_title", { amount: value }),
      description:
        full && payment.paid_order
          ? t("orders.refund.confirm_full_desc")
          : t("orders.refund.confirm_partial_desc"),
      confirmLabel: t("orders.refund.action"),
      variant: "destructive",
    });
    if (!ok) return;
    // One key per confirmed click: a retried request (step-up, network) never refunds twice.
    refundMutation.mutate({ payment, amountMinor, key: crypto.randomUUID() });
  };

  return (
    <div className="space-y-3">
      {data.payments.map((payment) => (
        <div key={payment.uuid} className="p-3.5 rounded-xl border text-xs space-y-2 bg-card/60">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="font-mono">
              {payment.provider} · {payment.provider_ref || payment.uuid.slice(0, 8)}
            </span>
            {!payment.paid_order && <Badge variant="outline">{t("orders.refund.extra_payment")}</Badge>}
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div>
              <div className="text-muted-foreground">{t("orders.refund.paid")}</div>
              <div className="font-mono">{money(payment.amount_minor, payment.currency)}</div>
            </div>
            <div>
              <div className="text-muted-foreground">{t("orders.refund.refunded")}</div>
              <div className="font-mono">{money(payment.refunded_minor, payment.currency)}</div>
            </div>
            <div>
              <div className="text-muted-foreground">{t("orders.refund.refundable")}</div>
              <div className="font-mono font-semibold">{money(payment.refundable_minor, payment.currency)}</div>
            </div>
          </div>
          {payment.refundable_minor > 0 && (
            <div className="grid grid-cols-1 sm:grid-cols-[8rem_1fr_auto] items-end gap-2 pt-1">
              <div className="space-y-1">
                <Label className="text-[11px]">{t("orders.refund.amount_label")}</Label>
                <Input
                  inputMode="decimal"
                  placeholder={(payment.refundable_minor / 100).toFixed(2)}
                  value={amount}
                  onChange={(e) => setAmount(e.target.value)}
                />
              </div>
              <div className="space-y-1">
                <Label className="text-[11px]">{t("orders.refund.reason_label")}</Label>
                <Input value={reason} maxLength={500} onChange={(e) => setReason(e.target.value)} />
              </div>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => startRefund(payment)}
                disabled={refundMutation.isPending || payment.pending_minor > 0}
              >
                <Undo2 className="mr-1.5 h-3.5 w-3.5" />
                {t("orders.refund.action")}
              </Button>
            </div>
          )}
        </div>
      ))}

      {data.refunds.length > 0 && (
        <div className="space-y-2">
          <h5 className="text-xs font-semibold">{t("orders.refund.history_heading")}</h5>
          {data.refunds.map((refund) => (
            <div key={refund.uuid} className="p-3 rounded-lg border text-xs space-y-1">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-mono">{money(refund.amount_minor, refund.currency)}</span>
                <Badge
                  variant={
                    refund.status === "succeeded" ? "success" : refund.status === "failed" ? "danger" : "outline"
                  }
                >
                  {t(`orders.refund.status_${refund.status}`)}
                </Badge>
              </div>
              <div className="text-muted-foreground">
                {new Date(refund.created_at).toLocaleString(locale === "tr" ? "tr-TR" : "en-US")}
                {refund.attempts > 1 && ` · ${t("orders.refund.attempts", { count: refund.attempts })}`}
                {refund.reason && ` · ${refund.reason}`}
              </div>
              {refund.status === "failed" && (
                <div className="flex flex-wrap items-start justify-between gap-2 rounded-md border border-destructive/30 bg-destructive/5 p-2">
                  <div className="space-y-1">
                    <p className="text-destructive">{refund.error_message}</p>
                    {refund.error_code === "provider_error" && (
                      <p className="flex items-center gap-1 text-amber-600 dark:text-amber-400">
                        <AlertTriangle className="h-3.5 w-3.5" /> {t("orders.refund.unknown_outcome_hint")}
                      </p>
                    )}
                  </div>
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => retryMutation.mutate(refund)}
                    disabled={retryMutation.isPending}
                  >
                    <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                    {t("orders.refund.retry")}
                  </Button>
                </div>
              )}
              {refund.status === "pending" && (
                <p className="text-muted-foreground">{t("orders.refund.pending_hint")}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
