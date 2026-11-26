import { assertServiceMediaURL } from "@/lib/media/urls";

/** ComCode default card artwork (TEC-1320), shown when a card face has no design image. */
export const DEFAULT_CARD_ART = {
  front: "/cards/default-card-front.svg",
  back: "/cards/default-card-back.svg",
} as const;

export type CardSide = keyof typeof DEFAULT_CARD_ART;

/** The API-owned design image of a card face, or null for the default artwork. */
export function cardFaceImage(url: string | null | undefined): string | null {
  if (!url) return null;
  try {
    return assertServiceMediaURL(url);
  } catch {
    return null;
  }
}
