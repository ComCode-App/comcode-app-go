"use client";

import { Upload } from "lucide-react";
import * as React from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils/index";
import { useLocale } from "@/providers/locale-provider";

export type FileInputProps = Omit<
  React.InputHTMLAttributes<HTMLInputElement>,
  "type" | "value" | "defaultValue"
> & {
  /** Button text; defaults to the localized "Choose file". */
  buttonLabel?: React.ReactNode;
  /** Show the picked file name (or "No file chosen") next to the button. */
  showFileName?: boolean;
  buttonVariant?: React.ComponentProps<typeof Button>["variant"];
  buttonSize?: React.ComponentProps<typeof Button>["size"];
  /** Classes for the wrapper. */
  className?: string;
};

/**
 * App-styled file picker: a hidden `<input type="file">` opened by a Button,
 * so the browser's "Choose file / No file chosen" control never shows
 * (TEC-1275). The ref, `id`, `onChange` and the rest go to the real input,
 * so callers keep reading `event.target.files` and resetting `ref.value`.
 */
export const FileInput = React.forwardRef<HTMLInputElement, FileInputProps>(
  (
    {
      buttonLabel,
      showFileName = true,
      buttonVariant = "outline",
      buttonSize = "sm",
      className,
      onChange,
      disabled,
      id,
      ...props
    },
    ref,
  ) => {
    const { t } = useLocale();
    const innerRef = React.useRef<HTMLInputElement | null>(null);
    const [names, setNames] = React.useState<string[]>([]);
    const generatedId = React.useId();
    const inputId = id ?? generatedId;

    const setRefs = React.useCallback(
      (node: HTMLInputElement | null) => {
        innerRef.current = node;
        if (typeof ref === "function") ref(node);
        else if (ref) ref.current = node;
      },
      [ref],
    );

    return (
      <div
        data-slot="file-input"
        className={cn("flex min-w-0 items-center gap-2", className)}
      >
        <input
          {...props}
          ref={setRefs}
          id={inputId}
          type="file"
          disabled={disabled}
          className="sr-only"
          tabIndex={-1}
          onChange={(event) => {
            setNames(Array.from(event.target.files ?? [], (f) => f.name));
            onChange?.(event);
          }}
        />
        <Button
          type="button"
          variant={buttonVariant}
          size={buttonSize}
          disabled={disabled}
          aria-controls={inputId}
          onClick={() => innerRef.current?.click()}
        >
          <Upload aria-hidden />
          {buttonLabel ?? t("common.file_choose")}
        </Button>
        {showFileName ? (
          <span
            className="text-muted-foreground min-w-0 truncate text-xs"
            aria-live="polite"
          >
            {names.length ? names.join(", ") : t("common.file_none")}
          </span>
        ) : null}
      </div>
    );
  },
);
FileInput.displayName = "FileInput";
