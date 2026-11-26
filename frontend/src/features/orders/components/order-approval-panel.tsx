"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CheckCircle2, Clock, XCircle } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { permissions } from "@/config/permissions";
import { isApiError } from "@/lib/api";
import { useDialogs } from "@/providers/dialog-provider";
import { useLocale } from "@/providers/locale-provider";
import { usePermission } from "@/providers/permission-provider";
import { ordersQueryKeys } from "../hooks/query-keys";
import { ordersService, type OrderDetail } from "../services/orders.service";

/**
 * Approve / reject a company bulk order awaiting approval (TEC-1317). Approval opens payment
 * for the customer; it is blocked while the company is over its profile quota. Rejection needs
 * a reason and deletes the draft profiles the order created.
 */
export function OrderApprovalPanel({ order }: { order: OrderDetail }) {
  const { t } = useLocale();
  const { confirm } = useDialogs();
  const canApprove = usePermission().can(permissions.orders.approve);
  const queryClient = useQueryClient();
  const [rejectOpen, setRejectOpen] = useState(false);
  const [reason, setReason] = useState("");

  const drafts = order.items.filter((i) => i.profile_created_by_order).length;
  const linked = order.items.filter(
    (i) => i.profile_uuid && !i.profile_created_by_order,
  ).length;

  const onDone = () =>
    queryClient.invalidateQueries({ queryKey: ordersQueryKeys.all });

  const approve = useMutation({
    mutationFn: () => ordersService.approve(order.uuid),
    onSuccess: () => {
      toast.success(t("orders.approval.approved_toast"));
      void onDone();
    },
    onError: (err) => {
      if (isApiError(err) && err.code === "PROFILE_QUOTA_EXCEEDED") {
        const missing =
          err.details.find((d) => d.field === "missing")?.message ?? "?";
        toast.error(t("orders.approval.quota_exceeded", { missing }));
        return;
      }
      toast.error(isApiError(err) ? err.message : t("orders.approval.error"));
    },
  });

  const reject = useMutation({
    mutationFn: () => ordersService.reject(order.uuid, reason.trim()),
    onSuccess: () => {
      toast.success(t("orders.approval.rejected_toast"));
      setRejectOpen(false);
      setReason("");
      void onDone();
    },
    onError: (err) =>
      toast.error(isApiError(err) ? err.message : t("orders.approval.error")),
  });

  const handleApprove = async () => {
    const ok = await confirm({
      title: t("orders.approval.approve_confirm_title"),
      description: t("orders.approval.approve_confirm_desc", {
        order: order.order_number,
      }),
      confirmLabel: t("orders.approval.approve"),
    });
    if (ok) approve.mutate();
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-violet-500/30 bg-violet-500/5 p-4">
      <div className="space-y-0.5">
        <p className="flex items-center gap-1.5 text-sm font-semibold">
          <Clock className="h-4 w-4 text-violet-600" />
          {t("orders.approval.panel_title")}
        </p>
        <p className="text-muted-foreground text-xs">
          {t("orders.approval.panel_desc", { drafts, linked })}
        </p>
      </div>
      {canApprove ? (
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5"
            disabled={reject.isPending || approve.isPending}
            onClick={() => setRejectOpen(true)}
          >
            <XCircle className="h-3.5 w-3.5" />
            {t("orders.approval.reject")}
          </Button>
          <Button
            size="sm"
            className="gap-1.5"
            disabled={approve.isPending || reject.isPending}
            onClick={() => void handleApprove()}
          >
            <CheckCircle2 className="h-3.5 w-3.5" />
            {t("orders.approval.approve")}
          </Button>
        </div>
      ) : null}

      <Dialog open={rejectOpen} onOpenChange={setRejectOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("orders.approval.reject_title")}</DialogTitle>
            <DialogDescription>
              {t("orders.approval.reject_desc")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="order-reject-reason">
              {t("orders.approval.reason_label")}
            </Label>
            <Textarea
              id="order-reject-reason"
              value={reason}
              rows={3}
              maxLength={2000}
              placeholder={t("orders.approval.reason_placeholder")}
              onChange={(e) => setReason(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRejectOpen(false)}>
              {t("common.cancel")}
            </Button>
            <Button
              variant="destructive"
              disabled={!reason.trim() || reject.isPending}
              onClick={() => reject.mutate()}
            >
              {t("orders.approval.reject")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
