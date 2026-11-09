"use client";

import { type ReactNode, useEffect, useState } from "react";

import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

/** Search box state: the raw input and its trimmed value 300ms after the last keystroke. */
export function useDebouncedQuery() {
  const [input, setInput] = useState("");
  const [q, setQ] = useState("");
  useEffect(() => {
    const id = setTimeout(() => setQ(input.trim()), 300);
    return () => clearTimeout(id);
  }, [input]);
  return { input, setInput, q };
}

export function Field({ label, value, mono }: { label: string; value?: string; mono?: boolean }) {
  return (
    <div className="flex min-w-0 gap-2">
      <dt className="text-muted-foreground shrink-0">{label}:</dt>
      <dd className={cn("truncate", mono && "font-mono text-xs leading-5")}>{value || "—"}</dd>
    </div>
  );
}

export function Empty({ icon, text }: { icon: ReactNode; text: string }) {
  return (
    <Card>
      <CardContent className="text-muted-foreground py-12 text-center text-sm">
        {icon}
        {text}
      </CardContent>
    </Card>
  );
}
