import type { ComponentProps, ReactNode } from "react";

import { FieldDescription } from "@/components/ui/field";
import { cn } from "@/lib/utils";

export function AuthCard({
  title,
  description,
  children,
  footer,
  className,
  ...props
}: ComponentProps<"div"> & {
  title: string;
  description?: string;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <div className={cn("flex flex-col gap-6", className)} {...props}>
      <div className="relative overflow-hidden rounded-3xl border border-white/60 bg-white/70 p-6 shadow-2xl shadow-purple-950/10 backdrop-blur-2xl backdrop-saturate-150 ring-1 ring-black/5 dark:border-white/10 dark:bg-black/35 dark:shadow-purple-950/50 dark:ring-white/10">
        {/* Top edge subtle specular highlight */}
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/80 to-transparent dark:via-white/20"
        />

        <div className="mb-6 flex flex-col gap-1.5 text-center">
          <h2 className="text-xl font-semibold tracking-tight text-foreground">
            {title}
          </h2>
          {description ? (
            <p className="text-sm text-muted-foreground">{description}</p>
          ) : null}
        </div>

        <div>{children}</div>
      </div>

      {footer ? (
        <FieldDescription className="px-6 text-center text-xs text-muted-foreground">
          {footer}
        </FieldDescription>
      ) : null}
    </div>
  );
}


