import { proxyToUpstream } from "@/lib/server/bff-proxy";

const segments = ["public", "payment", "iyzico", "callback"] as const;

async function handle(request: Request) {
  return proxyToUpstream([...segments], request);
}

export const GET = handle;
export const POST = handle;
