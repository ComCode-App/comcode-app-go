import type { LucideIcon } from "lucide-react";

export type ServiceHealthStatus = "healthy" | "degraded" | "down" | "disabled";

export type MemoryStats = {
  alloc_bytes: number;
  total_alloc_bytes: number;
  sys_bytes: number;
  heap_alloc_bytes: number;
  heap_sys_bytes: number;
  heap_idle_bytes: number;
  heap_inuse_bytes: number;
  num_gc: number;
};

export type RuntimeInfo = {
  go_version: string;
  num_cpu: number;
  num_goroutine: number;
  compiler: string;
  arch: string;
  os: string;
  memory: MemoryStats;
  uptime_seconds: number;
  server_time: string;
};

export type PoolStats = {
  total_conns: number;
  idle_conns: number;
  acquired_conns: number;
  max_conns: number;
};

export type ServiceHealth = {
  status: ServiceHealthStatus;
  latency_ms: number;
  message?: string;
};

export type DatabaseHealth = ServiceHealth & {
  pool?: PoolStats;
};

export type StorageHealth = ServiceHealth & {
  driver: string;
};

export type SystemServicesHealth = {
  database: DatabaseHealth;
  redis: ServiceHealth;
  storage: StorageHealth;
  meilisearch: ServiceHealth;
};

export type EnvironmentInfo = {
  app_name: string;
  env: string;
  log_level: string;
  centrifugo_enabled: boolean;
  vapid_enabled: boolean;
  search_enabled: boolean;
};

export type SystemInfo = {
  status: "healthy" | "degraded" | "down";
  runtime: RuntimeInfo;
  services: SystemServicesHealth;
  environment: EnvironmentInfo;
};

export type SettingsNavItem = {
  id: string;
  titleKey: string;
  href: string;
  icon: LucideIcon;
  badge?: string;
  permission?: string;
  external?: boolean;
};

export type SettingsNavGroup = {
  id: string;
  labelKey: string;
  items: SettingsNavItem[];
};
