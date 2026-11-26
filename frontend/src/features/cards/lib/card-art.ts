import { assertServiceMediaURL } from "@/lib/media/urls";

/** ComCode default card artwork (TEC-1320), shown when a card face has no design image. */
export const DEFAULT_CARD_ART = {
  front: "/cards/default-card-front.svg",
  back: "/cards/default-card-back.svg",
} as const;

/** ComCode default badge (yaka kartı) artwork, portrait 54×85.6 mm (TEC-1320). */
export const DEFAULT_BADGE_ART = {
  front: "/cards/default-badge-front.svg",
  back: "/cards/default-badge-back.svg",
} as const;

export type CardSide = keyof typeof DEFAULT_CARD_ART;

/** card_type of a badge card; provisioning sets it for products of type "badge". */
export const BADGE_CARD_TYPE = "badge";
/** product_type of the badge (yaka kartı) product. */
export const BADGE_PRODUCT_TYPE = "badge";

/** A card is drawn as a portrait badge (54×85.6 mm) or a landscape card (85.6×54 mm). */
export type CardShape = "card" | "badge";

export function cardShapeOf(cardType: string | null | undefined): CardShape {
  return cardType === BADGE_CARD_TYPE ? "badge" : "card";
}

export function productShapeOf(
  productType: string | null | undefined,
): CardShape {
  return productType === BADGE_PRODUCT_TYPE ? "badge" : "card";
}

/** Default artwork of a card face for the shape. */
export function defaultArt(shape: CardShape, side: CardSide): string {
  return (shape === "badge" ? DEFAULT_BADGE_ART : DEFAULT_CARD_ART)[side];
}

/** Tailwind aspect ratio class of the shape. */
export function shapeAspectClass(shape: CardShape): string {
  return shape === "badge" ? "aspect-[54/85.6]" : "aspect-[1.586/1]";
}

/** The API-owned design image of a card face, or null for the default artwork. */
export function cardFaceImage(url: string | null | undefined): string | null {
  if (!url) return null;
  try {
    return assertServiceMediaURL(url);
  } catch {
    return null;
  }
}
