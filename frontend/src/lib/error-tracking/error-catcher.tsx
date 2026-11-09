"use client";

import { installErrorCatcher } from "./catcher";

// Module scope: listen as soon as the chunk runs, before hydration finishes.
installErrorCatcher();

/** Lazy error catcher for every page (root layout). Renders nothing. */
export function ErrorCatcher() {
  return null;
}
