import { proxyToUpstream } from "@/lib/server/bff-proxy";

const segments = ["public", "payment", "paytr", "callback"] as const;

async function handle(request: Request) {
  return proxyToUpstream([...segments], request);
}

export const POST = handle;
