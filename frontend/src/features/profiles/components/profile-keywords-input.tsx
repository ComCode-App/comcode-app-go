"use client";

import { useState } from "react";
import { X } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { useLocale } from "@/providers/locale-provider";
import {
  KEYWORDS_MAX_COUNT,
  addKeywords,
  joinKeywords,
  parseKeywords,
} from "../lib/keywords";

type ProfileKeywordsInputProps = {
  id?: string;
  /** Stored value: the translation note, comma-separated (TEC-1304). */
  value: string;
  onChange: (value: string) => void;
};

/** Tag-style editor for profile keywords: Enter or comma adds, × removes. */
export function ProfileKeywordsInput({
  id,
  value,
  onChange,
}: ProfileKeywordsInputProps) {
  const { t } = useLocale();
  const [draft, setDraft] = useState("");
  const keywords = parseKeywords(value);
  const full = keywords.length >= KEYWORDS_MAX_COUNT;

  const commit = (input: string) => {
    if (!input.trim()) return;
    onChange(joinKeywords(addKeywords(keywords, input)));
    setDraft("");
  };

  const remove = (index: number) => {
    onChange(joinKeywords(keywords.filter((_, i) => i !== index)));
  };

  return (
    <div className="space-y-3">
      <Input
        id={id}
        value={draft}
        disabled={full}
        placeholder={
          full
            ? t("profiles.keywords.limit_reached", {
                count: KEYWORDS_MAX_COUNT,
              })
            : t("profiles.keywords.placeholder")
        }
        onChange={(e) => {
          const next = e.target.value;
          if (/[,;]/.test(next)) {
            commit(next);
          } else {
            setDraft(next);
          }
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.preventDefault();
            commit(draft);
          } else if (e.key === "Backspace" && !draft && keywords.length > 0) {
            remove(keywords.length - 1);
          }
        }}
        onBlur={() => commit(draft)}
      />

      {keywords.length === 0 ? (
        <p className="text-muted-foreground text-xs">
          {t("profiles.keywords.empty")}
        </p>
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {keywords.map((keyword, index) => (
            <Badge
              key={keyword}
              variant="secondary"
              className="gap-1 py-1 pr-1 pl-2.5 text-xs"
            >
              {keyword}
              <button
                type="button"
                onClick={() => remove(index)}
                aria-label={t("profiles.keywords.remove", { keyword })}
                className="hover:bg-muted-foreground/20 rounded-full p-0.5"
              >
                <X className="h-3 w-3" />
              </button>
            </Badge>
          ))}
        </div>
      )}
    </div>
  );
}
