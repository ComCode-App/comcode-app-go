"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { FileUp, Send } from "lucide-react";
import { useOrderMutations } from "../hooks/use-orders";
import { useLocale } from "@/providers/locale-provider";

interface OrderRequestAssetDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  orderUuid: string;
  orderNumber: string;
}

export function OrderRequestAssetDialog({
  open,
  onOpenChange,
  orderUuid,
  orderNumber,
}: OrderRequestAssetDialogProps) {
  const { t } = useLocale();
  const { createAssetRequestMutation } = useOrderMutations();
  const [requestType, setRequestType] = useState("company_logo");
  const [title, setTitle] = useState(() => t("orders.asset_modal.default_title_logo"));
  const [description, setDescription] = useState(() => t("orders.asset_modal.default_desc_logo"));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await createAssetRequestMutation.mutateAsync({
      orderUuid,
      payload: {
        request_type: requestType,
        title,
        description,
      },
    });
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileUp className="h-5 w-5 text-primary" />
            {t("orders.asset_modal.title")}
          </DialogTitle>
          <DialogDescription>
            {t("orders.asset_modal.description", { orderNumber })}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 pt-2">
          <div className="space-y-2">
            <Label>{t("orders.asset_modal.type_label")}</Label>
            <Select
              value={requestType}
              onValueChange={(val) => {
                setRequestType(val);
                if (val === "company_logo") {
                  setTitle(t("orders.asset_modal.default_title_logo"));
                  setDescription(t("orders.asset_modal.default_desc_logo"));
                } else if (val === "vector_artwork") {
                  setTitle(t("orders.asset_modal.default_title_artwork"));
                  setDescription(t("orders.asset_modal.default_desc_artwork"));
                } else if (val === "custom_font") {
                  setTitle(t("orders.asset_modal.default_title_font"));
                  setDescription(t("orders.asset_modal.default_desc_font"));
                }
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="company_logo">{t("orders.asset_modal.type_company_logo")}</SelectItem>
                <SelectItem value="vector_artwork">{t("orders.asset_modal.type_artwork")}</SelectItem>
                <SelectItem value="custom_font">{t("orders.asset_modal.type_font")}</SelectItem>
                <SelectItem value="design_brief">{t("orders.asset_modal.type_brief")}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("orders.asset_modal.title_label")}</Label>
            <Input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          </div>

          <div className="space-y-2">
            <Label>{t("orders.asset_modal.desc_label")}</Label>
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              required
            />
          </div>

          <DialogFooter className="pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={createAssetRequestMutation.isPending}
            >
              {t("common.cancel")}
            </Button>
            <Button type="submit" disabled={createAssetRequestMutation.isPending}>
              <Send className="mr-2 h-4 w-4" />
              {t("orders.asset_modal.send_btn")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
