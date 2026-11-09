import { handleProxyRequest } from "@/lib/server/bff-proxy";

const segments = ["public", "payment", "iyzico", "callback"] as const;

// Cross-site POST from the payment provider: exempt from the BFF CSRF check
// (see CSRF_EXEMPT_PATHS in lib/server/bff-guard.ts).
async function handle(request: Request) {
  return handleProxyRequest([...segments], request);
}

export const GET = handle;
export const POST = handle;
