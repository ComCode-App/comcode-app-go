import { i18nConfig, type AppLocale } from "@/config/i18n";
import { translate } from "@/lib/i18n/messages";

import { resolveApiErrorMessage } from "./error-messages";

export type ApiErrorDetail = {
  field?: string;
  message?: string;
  code?: string;
};

export type ApiErrorBody = {
  success: false;
  error: {
    code: string;
    message: string;
    details?: ApiErrorDetail[];
  };
  meta?: {
    request_id?: string;
  };
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly details: ApiErrorDetail[];
  readonly requestId?: string;
  readonly body?: ApiErrorBody;
  /** The API's own (untranslated) message; `message` is the localized text. */
  readonly serverMessage?: string;

  constructor(opts: {
    status: number;
    code: string;
    message: string;
    details?: ApiErrorDetail[];
    requestId?: string;
    body?: ApiErrorBody;
    serverMessage?: string;
  }) {
    super(opts.message);
    this.name = "ApiError";
    this.status = opts.status;
    this.code = opts.code;
    this.serverMessage = opts.serverMessage;
    this.details = opts.details ?? [];
    this.requestId = opts.requestId;
    this.body = opts.body;
  }

  get isUnauthorized() {
    return this.status === 401;
  }

  get isForbidden() {
    return this.status === 403;
  }

  /** Plan quota used up (403 PLAN_LIMIT_EXCEEDED); see `planLimitFeature`. */
  get isPlanLimit() {
    return this.code === "PLAN_LIMIT_EXCEEDED";
  }

  /** Feature key of a plan-limit error, e.g. `profiles.max`. */
  get planLimitFeature(): string | undefined {
    return this.isPlanLimit ? this.details[0]?.field : undefined;
  }

  get isNotFound() {
    return this.status === 404;
  }

  get isValidation() {
    return this.status === 422 || this.status === 400;
  }

  get isServer() {
    return this.status >= 500;
  }

  fieldErrors(): Record<string, string> {
    const map: Record<string, string> = {};
    for (const detail of this.details) {
      if (detail.field && detail.message) {
        map[detail.field] = detail.message;
      }
    }
    return map;
  }
}

export function parseApiError(status: number, data: unknown): ApiError {
  const body = data as Partial<ApiErrorBody> | null;
  const code = body?.error?.code ?? `HTTP_${status}`;
  const serverMessage = body?.error?.message;
  return new ApiError({
    status,
    code,
    message: localizedMessage(code, status, serverMessage),
    serverMessage,
    details: body?.error?.details ?? [],
    requestId: body?.meta?.request_id,
    body: body as ApiErrorBody | undefined,
  });
}

/** The language the page is shown in (LocaleProvider keeps `<html lang>` in sync). */
function activeLocale(): AppLocale {
  if (typeof document !== "undefined") {
    const lang = document.documentElement.lang;
    if ((i18nConfig.supportedLocales as readonly string[]).includes(lang)) {
      return lang as AppLocale;
    }
  }
  return i18nConfig.defaultLocale;
}

function localizedMessage(
  code: string,
  status: number,
  serverMessage?: string,
) {
  const locale = activeLocale();
  return resolveApiErrorMessage({
    code,
    status,
    serverMessage,
    locale,
    translate: (key) => {
      const text = translate(locale, key);
      return text === key ? undefined : text;
    },
  });
}

export type ErrorHandler = (error: ApiError) => void;

let globalErrorHandler: ErrorHandler | null = null;

export function setGlobalApiErrorHandler(handler: ErrorHandler | null) {
  globalErrorHandler = handler;
}

// One failed request reaches several layers (unwrap, the react-query caches, the caller's own
// onError); each error is toasted at most once. The toast is deferred by a macrotask so a layer
// that shows the error itself can still call suppressApiErrorToast() in the same rejection chain.
const emittedErrors = new WeakSet<ApiError>();
const suppressedErrors = new WeakSet<ApiError>();

/** Queues the global toast for an API error; repeated calls for the same error are ignored. */
export function emitApiError(error: ApiError) {
  if (emittedErrors.has(error) || suppressedErrors.has(error)) return;
  emittedErrors.add(error);
  // The step-up dialog handles it; a toast would only repeat the prompt.
  if (error.code === "STEP_UP_REQUIRED") return;
  setTimeout(() => {
    if (!suppressedErrors.has(error)) globalErrorHandler?.(error);
  }, 0);
}

/**
 * Opt out of the global toast for this error because the caller shows it (inline or with its
 * own toast). Safe to call from a catch block or onError handler of the failing request.
 */
export function suppressApiErrorToast(error: unknown) {
  if (error instanceof ApiError) suppressedErrors.add(error);
}

/** True for 403 PLAN_LIMIT_EXCEEDED; the global toast already offers the upgrade. */
export function isPlanLimitError(error: unknown): boolean {
  return error instanceof ApiError && error.isPlanLimit;
}
