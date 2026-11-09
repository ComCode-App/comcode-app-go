import { handleMobileGatewayRequest } from "@/lib/server/mobile-gateway";

type RouteContext = {
  params: Promise<{ path: string[] }>;
};

/** Long-lived SSE streams; aligned with the cookie BFF and API `HTTP_WRITE_TIMEOUT`. */
export const maxDuration = 120;

/**
 * Mobile API gateway (TEC-1178): Bearer only, never cookies; tokens stay in
 * the JSON. See `lib/server/mobile-gateway.ts`.
 */
async function handle(request: Request, context: RouteContext) {
  const { path } = await context.params;
  return handleMobileGatewayRequest(path ?? [], request);
}

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
export const HEAD = handle;
export const OPTIONS = handle;
