"use client";

import Link from "next/link";
import type { ReactNode } from "react";

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/common/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

type DashboardStatCardProps = {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  href?: string;
  loading?: boolean;
  className?: string;
};

export function DashboardStatCard({
  label,
  value,
  hint,
  href,
  loading = false,
  className,
}: DashboardStatCardProps) {
  const body = (
    <>
      <CardHeader className="pb-1">
        <CardTitle className="text-muted-foreground text-sm font-medium">
          {label}
        </CardTitle>
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <Skeleton className="h-8 w-20" />
        ) : (
          <>
            <p className="text-2xl font-semibold tracking-tight tabular-nums">
              {value}
            </p>
            {hint ? (
              <p className="text-muted-foreground mt-1 text-xs">{hint}</p>
            ) : null}
          </>
        )}
      </CardContent>
    </>
  );

  if (href) {
    return (
      <Card
        className={cn(
          "hover:border-primary/40 hover:bg-muted/30 shadow-none transition-colors",
          className,
        )}
      >
        <Link href={href} className="block focus-visible:outline-none">
          {body}
        </Link>
      </Card>
    );
  }

  return <Card className={cn("shadow-none", className)}>{body}</Card>;
}
