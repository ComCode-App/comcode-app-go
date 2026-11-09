"use client";

import { brand } from "@/config/brand";

import { BrandMark } from "./brand-image";

type AppMarkProps = {
  className?: string;
  title?: string;
};

/** @deprecated Prefer BrandMark — kept for existing imports. */
export function AppMark({ className, title = brand.name }: AppMarkProps) {
  return <BrandMark className={className} title={title} />;
}
