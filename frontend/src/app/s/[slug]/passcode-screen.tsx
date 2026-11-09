"use client";

import { KeyRound } from "lucide-react";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { SmartLinkResolution } from "@/features/cards/services/cards.service";
import { withVia } from "@/features/leads/lib/via";
import { useLocale } from "@/providers/locale-provider";

/**
 * Passcode entry for a protected smart link (TEC-556). The passcode goes in the POST body only,
 * never in the URL; the API rate limits wrong attempts.
 */
export function SmartLinkPasscodeScreen({ slug }: { slug: string }) {
  const { t } = useLocale();
  const [passcode, setPasscode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!passcode) return;
    setPending(true);
    setError(null);
    try {
      const res = await fetch(`/api/v1/s/${encodeURIComponent(slug)}/unlock`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({ passcode }),
        cache: "no-store",
      });
      if (res.ok) {
        const { data } = (await res.json()) as { data?: SmartLinkResolution };
        if (data?.target_url) {
          // replace: the passcode screen should not come back on "back".
          window.location.replace(withVia(data.target_url, `s/${slug}`));
          return;
        }
        setError(t("public_profile.link_passcode.gone"));
      } else if (res.status === 403) {
        setError(t("public_profile.link_passcode.wrong"));
        setPasscode("");
      } else if (res.status === 429) {
        setError(t("public_profile.link_passcode.rate_limited"));
      } else if (res.status === 404) {
        setError(t("public_profile.link_passcode.gone"));
      } else {
        setError(t("public_profile.link_passcode.error"));
      }
    } catch {
      setError(t("public_profile.link_passcode.error"));
    }
    setPending(false);
  };

  return (
    <main className="bg-background flex min-h-dvh items-center justify-center px-4">
      <form onSubmit={submit} className="w-full max-w-sm space-y-4 text-center">
        <span className="bg-muted mx-auto flex size-14 items-center justify-center rounded-full">
          <KeyRound className="text-muted-foreground size-6" />
        </span>
        <div>
          <h1 className="text-xl font-semibold">
            {t("public_profile.link_passcode.title")}
          </h1>
          <p className="text-muted-foreground mt-1 text-sm">
            {t("public_profile.link_passcode.body")}
          </p>
        </div>
        <div className="grid gap-1.5 text-left">
          <Label htmlFor="smart-link-passcode">
            {t("public_profile.link_passcode.label")}
          </Label>
          <Input
            id="smart-link-passcode"
            type="password"
            value={passcode}
            onChange={(e) => setPasscode(e.target.value)}
            autoComplete="off"
            autoFocus
            maxLength={64}
            aria-invalid={Boolean(error)}
          />
        </div>
        {error ? (
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        ) : null}
        <Button
          type="submit"
          className="w-full"
          disabled={pending || !passcode}
        >
          {t("public_profile.link_passcode.submit")}
        </Button>
      </form>
    </main>
  );
}
