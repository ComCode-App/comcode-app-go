import Link from "next/link";
import type { ReactNode } from "react";

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/common/card";
import { Skeleton } from "@/components/ui/skeleton";

type OverviewSectionProps = {
  title: string;
  description?: string;
  /** Optional link to the full list behind this section. */
  action?: { href: string; label: string };
  /** Optional control next to the title (e.g. a period picker). */
  toolbar?: ReactNode;
  loading?: boolean;
  className?: string;
  children: ReactNode;
};

/** Card chrome shared by the overview sections. */
export function OverviewSection({
  title,
  description,
  action,
  toolbar,
  loading,
  className,
  children,
}: OverviewSectionProps) {
  return (
    <Card className={className ? `shadow-none ${className}` : "shadow-none"}>
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0 pb-3">
        <div className="min-w-0">
          <CardTitle className="text-sm font-semibold">{title}</CardTitle>
          {description ? (
            <p className="text-muted-foreground mt-1 text-xs">{description}</p>
          ) : null}
        </div>
        {toolbar || action ? (
          <div className="flex shrink-0 items-center gap-3">
            {toolbar}
            {action ? (
              <Link
                href={action.href}
                className="text-primary text-xs font-medium hover:underline"
              >
                {action.label}
              </Link>
            ) : null}
          </div>
        ) : null}
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-5/6" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        ) : (
          children
        )}
      </CardContent>
    </Card>
  );
}
