"use client";

import { useEffect } from "react";

import { enableFullErrorTracking } from "./catcher";

/** Panel / admin shell: loads the full SDK on mount (TEC-952). Renders nothing. */
export function FullErrorTracking() {
  useEffect(() => {
    void enableFullErrorTracking();
  }, []);
  return null;
}
