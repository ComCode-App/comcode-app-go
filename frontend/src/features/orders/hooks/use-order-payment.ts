"use client";

import { useMutation } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { routes } from "@/config/routes";
import { useAuthStore } from "@/lib/auth/session-store";
import { ordersService } from "../services/orders.service";

/** Owners and admins of the active account pay orders (backend: account.CanWrite). */
export function useCanPayOrders() {
  const role = useAuthStore((s) => s.user?.account?.role);
  return role === "owner" || role === "admin";
}

/** Starts an online payment for an order and opens its checkout page (TEC-943). */
export function useStartOrderPayment() {
  const router = useRouter();
  return useMutation({
    mutationFn: (orderUuid: string) => ordersService.startCheckout(orderUuid),
    onSuccess: (session) => {
      router.push(routes.app.orders.checkout(session.uuid));
    },
  });
}
