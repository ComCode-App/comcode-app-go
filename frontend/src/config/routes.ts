export const routes = {
  public: {
    root: "/",
    health: "/health",
    share: {
      slug: (slug: string) => `/share/${slug}`,
      signed: (token: string) => `/share/s/${token}`,
    },
  },
  guest: {
    login: "/platform/login",
    register: "/platform/register",
    forgotPassword: "/platform/forgot-password",
    resetPassword: "/platform/reset-password",
    verifyEmail: "/platform/verify-email",
    /** Company invite link (TEC-945); the API emails this path. */
    inviteRoot: "/platform/invite",
    invite: (token: string) => `/platform/invite/${token}`,
  },
  app: {
    root: "/",
    home: "/",
    profile: {
      root: "/profile",
      password: "/profile/password",
      preferences: "/profile/preferences",
    },
    profiles: {
      root: "/profiles",
      create: "/profiles/create",
      detail: (uuid: string) => `/profiles/${uuid}`,
      edit: (uuid: string) => `/profiles/${uuid}/edit`,
    },
    cards: {
      root: "/cards",
    },
    orders: {
      root: "/orders",
      checkout: (sessionUuid: string) => `/orders/checkout/${sessionUuid}`,
    },
    bankAccounts: {
      root: "/bank-accounts",
    },
    leads: {
      root: "/leads",
    },
    webhooks: {
      root: "/webhooks",
    },
    smartLinks: {
      root: "/smart-links",
    },
    analytics: {
      root: "/analytics",
    },
    account: {
      root: "/account",
    },
    /** Company management lives under the reserved `/company` prefix (publicpath reserved words, TEC-949). */
    company: {
      root: "/company",
      offices: "/company/offices",
      groups: "/company/groups",
      members: "/company/members",
      bulk: "/company/bulk",
      analytics: "/company/analytics",
      branding: "/company/branding",
    },
    billing: {
      root: "/billing",
      checkout: (sessionUuid: string) => `/billing/checkout/${sessionUuid}`,
      addCard: (sessionUuid: string) => `/billing/cards/add/${sessionUuid}`,
      success: "/billing/success",
    },
    backgroundStudio: {
      root: "/background-studio",
    },
  },
  platform: {
    root: "/platform",
    home: "/platform",
    roles: {
      root: "/platform/settings/roles",
      create: "/platform/settings/roles/create",
      detail: (uuid: string) => `/platform/settings/roles/${uuid}`,
      edit: (uuid: string) => `/platform/settings/roles/${uuid}/edit`,
    },
    users: {
      root: "/platform/users",
      create: "/platform/users/create",
      detail: (uuid: string) => `/platform/users/${uuid}`,
      edit: (uuid: string) => `/platform/users/${uuid}/edit`,
    },
    notifications: {
      root: "/platform/notifications",
      detail: (uuid: string) => `/platform/notifications/${uuid}`,
    },
    products: "/platform/products",
    categories: "/platform/categories",
    analytics: {
      root: "/platform/analytics",
      events: "/platform/analytics/events",
      definitions: "/platform/analytics/definitions",
      config: "/platform/analytics/config",
    },
    orders: "/platform/orders",
    profileVerifications: "/platform/profile-verifications",
    ordersPipeline: "/platform/orders/pipeline",
    support: {
      cards: "/platform/support/cards",
      routes: "/platform/support/routes",
    },
    imports: {
      root: "/platform/settings/imports",
      detail: (uuid: string) => `/platform/settings/imports/${uuid}`,
    },
    exports: {
      root: "/platform/settings/exports",
      detail: (uuid: string) => `/platform/settings/exports/${uuid}`,
    },
    settings: {
      root: "/platform/settings",
      system: "/platform/settings/system",
      environment: "/platform/settings/environment",
      general: "/platform/settings/general",
      roles: "/platform/settings/roles",
      activity: "/platform/settings/activity",
      logs: "/platform/settings/logs",
      socialPlatforms: "/platform/settings/social-platforms",
      banks: "/platform/settings/banks",
      sectors: "/platform/settings/sectors",
      plans: "/platform/settings/plans",
      subscriptions: "/platform/settings/subscriptions",
      imports: "/platform/settings/imports",
      exports: "/platform/settings/exports",
      auth: "/platform/settings/auth",
      access: "/platform/settings/access",
      payment: "/platform/settings/payment",
      paytr: "/platform/settings/payment/paytr",
      iyzico: "/platform/settings/payment/iyzico",
      integrations: "/platform/settings/integrations",
      geoip: "/platform/settings/geoip",
      errorTracking: "/platform/settings/error-tracking",
      storage: "/platform/settings/storage",
      maintenance: "/platform/settings/maintenance",
    },
    access: {
      root: "/platform/settings/access",
    },
    authSettings: {
      root: "/platform/settings/auth",
    },
    integrations: {
      github: "/platform/settings/integrations",
      google: "/platform/settings/integrations",
      facebook: "/platform/settings/integrations",
      apple: "/platform/settings/integrations",
    },
    activity: {
      root: "/platform/settings/activity",
    },
    logs: {
      root: "/platform/settings/logs",
    },
    storage: {
      root: "/platform/settings/storage",
    },
    geoip: {
      root: "/platform/settings/geoip",
    },
    companies: {
      root: "/platform/companies",
      create: "/platform/companies/create",
      detail: (uuid: string) => `/platform/companies/${uuid}`,
      edit: (uuid: string) => `/platform/companies/${uuid}/edit`,
    },
    plans: {
      root: "/platform/settings/plans",
      create: "/platform/settings/plans/create",
      detail: (uuid: string) => `/platform/settings/plans/${uuid}`,
      edit: (uuid: string) => `/platform/settings/plans/${uuid}/edit`,
    },
    socialPlatforms: {
      root: "/platform/settings/social-platforms",
    },
    banks: {
      root: "/platform/settings/banks",
    },
    subscriptions: {
      root: "/platform/settings/subscriptions",
      create: "/platform/settings/subscriptions/create",
      detail: (uuid: string) => `/platform/settings/subscriptions/${uuid}`,
    },
    payment: {
      settings: "/platform/settings/payment",
      paytr: "/platform/settings/payment/paytr",
      iyzico: "/platform/settings/payment/iyzico",
    },
    profile: {
      root: "/platform/profile",
      password: "/platform/profile/password",
      preferences: "/platform/profile/preferences",
    },
  },
  errors: {
    unauthorized: "/unauthorized",
    forbidden: "/forbidden",
  },
} as const;

type NestedRoutes<T> = T extends string
  ? T
  : T extends (...args: never[]) => infer R
    ? R extends string
      ? R
      : never
    : T extends Record<string, unknown>
      ? NestedRoutes<T[keyof T]>
      : never;

export type AppRoute = NestedRoutes<typeof routes>;
