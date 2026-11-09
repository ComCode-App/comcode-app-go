import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query";

import { ApiError, emitApiError, suppressApiErrorToast } from "@/lib/api";

/**
 * Global API error toasts (TEC-1110): each failed request shows exactly one toast.
 *
 * - `unwrap()` queues the toast; the caches below confirm or cancel it, and `emitApiError`
 *   never shows the same error twice.
 * - Queries: `meta: { silentErrors: true }` skips the toast (the screen renders the error).
 * - Mutations: a mutation with its own `onError` reports the error itself, so no global toast;
 *   add `meta: { toastErrors: true }` when that onError only cleans up. `meta: { silentErrors: true }`
 *   always skips it. Plan-limit errors keep their upgrade toast unless silenced explicitly.
 * - Plain `await` callers: `suppressApiErrorToast(err)` in the catch block. `{ silent: true }` on
 *   `unwrap` / `platformRequest` only stops unwrap's own toast; the caches above still apply.
 */
export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failureCount, error) => {
          if (error instanceof ApiError) {
            if ([401, 403, 404, 422].includes(error.status)) return false;
          }
          return failureCount < 2;
        },
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: false,
      },
    },
    queryCache: new QueryCache({
      onError: (error, query) => {
        if (!(error instanceof ApiError)) return;
        if (query.meta?.silentErrors) suppressApiErrorToast(error);
        else emitApiError(error);
      },
    }),
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) => {
        if (!(error instanceof ApiError)) return;
        const meta = mutation.meta;
        const handledByCaller =
          Boolean(mutation.options.onError) &&
          !meta?.toastErrors &&
          !error.isPlanLimit;
        if (meta?.silentErrors || handledByCaller) suppressApiErrorToast(error);
        else emitApiError(error);
      },
    }),
  });
}
