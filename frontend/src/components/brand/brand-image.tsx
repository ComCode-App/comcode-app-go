"use client";

import Image from "next/image";
import { useState } from "react";

import { brand } from "@/config/brand";
import { cn } from "@/lib/utils";

type BrandImageProps = {
  src: string;
  alt: string;
  className?: string;
  width?: number;
  height?: number;
  priority?: boolean;
};

/** Renders a brand SVG/PNG from `public/brand/`. */
export function BrandImage({
  src,
  alt,
  className,
  width = 120,
  height = 40,
  priority = false,
}: BrandImageProps) {
  const [missing, setMissing] = useState(false);

  if (missing) {
    return null;
  }

  return (
    <Image
      src={src}
      alt={alt}
      width={width}
      height={height}
      priority={priority}
      className={cn("h-auto w-auto object-contain", className)}
      unoptimized={src.endsWith(".svg")}
      onError={() => setMissing(true)}
    />
  );
}

export function BrandMark({
  className,
  title = brand.name,
}: {
  className?: string;
  title?: string;
}) {
  return (
    <BrandImage
      src={brand.assets.mark}
      alt={title}
      width={16}
      height={16}
      className={cn("size-4 shrink-0", className)}
    />
  );
}

export function BrandLogoHorizontal({
  className,
  title = brand.productName,
  priority = false,
}: {
  className?: string;
  title?: string;
  priority?: boolean;
}) {
  return (
    <BrandImage
      src={brand.assets.logo}
      alt={title}
      width={168}
      height={36}
      priority={priority}
      className={cn("h-9 w-auto max-w-[12rem] dark:invert", className)}
    />
  );
}

export function BrandLogoVertical({
  className,
  title = brand.productName,
}: {
  className?: string;
  title?: string;
}) {
  return (
    <BrandImage
      src={brand.assets.logoVertical}
      alt={title}
      width={120}
      height={160}
      className={cn("h-24 w-auto dark:invert", className)}
    />
  );
}
