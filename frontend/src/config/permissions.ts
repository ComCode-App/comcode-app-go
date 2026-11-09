/**
 * Permission catalog aligned with platform RBAC seed permissions.
 */
export const Permission = {
  AppBillingWrite: "app.billing.write",
  AppCompanyStructureRead: "app.company_structure.read",
  AppCompanyStructureWrite: "app.company_structure.write",
  AppCompanyProfilesBulk: "app.company_profiles.bulk",
  AppCompanyProfilesImport: "app.company_profiles.import",
  AppCompanyAnalyticsRead: "app.company_analytics.read",
  AppAccountInvitesRead: "app.account_invites.read",
  AppAccountInvitesWrite: "app.account_invites.write",
  AppCompanyBrandingRead: "app.company_branding.read",
  AppCompanyBrandingWrite: "app.company_branding.write",
  PlatformSectorsRead: "platform.sectors.read",
  PlatformSectorsWrite: "platform.sectors.write",
  PlatformUsersRead: "platform.users.read",
  PlatformUsersWrite: "platform.users.write",
  PlatformUsersExport: "platform.users.export",
  PlatformUsersImport: "platform.users.import",
  PlatformRolesRead: "platform.roles.read",
  PlatformRolesWrite: "platform.roles.write",
  PlatformRolesExport: "platform.roles.export",
  PlatformRolesImport: "platform.roles.import",
  PlatformNotificationsRead: "platform.notifications.read",
  PlatformNotificationsReadAll: "platform.notifications.read_all",
  PlatformNotificationsExport: "platform.notifications.export",
  PlatformSettingsRead: "platform.settings.read",
  PlatformSettingsWrite: "platform.settings.write",
  PlatformActivityRead: "platform.activity.read",
  PlatformImportsRead: "platform.imports.read",
  PlatformExportsRead: "platform.exports.read",
  PlatformStorageRead: "platform.storage.read",
  PlatformStorageWrite: "platform.storage.write",
  PlatformLogsRead: "platform.logs.read",
  PlatformLogsWrite: "platform.logs.write",
  PlatformUsersBulkDisable: "platform.users.bulk.disable",
  PlatformUsersBulkEnable: "platform.users.bulk.enable",
  PlatformUsersImpersonate: "platform.users.impersonate",
  PlatformRolesBulkDelete: "platform.roles.bulk.delete",
  PlatformBulkRead: "platform.bulk.read",
  PlatformAccessRead: "platform.access.read",
  PlatformAccessWrite: "platform.access.write",
  PlatformIntegrationsGitHubRead: "platform.integrations.github.read",
  PlatformIntegrationsGitHubWrite: "platform.integrations.github.write",
  PlatformIntegrationsGoogleRead: "platform.integrations.google.read",
  PlatformIntegrationsGoogleWrite: "platform.integrations.google.write",
  PlatformIntegrationsFacebookRead: "platform.integrations.facebook.read",
  PlatformIntegrationsFacebookWrite: "platform.integrations.facebook.write",
  PlatformIntegrationsAppleRead: "platform.integrations.apple.read",
  PlatformIntegrationsAppleWrite: "platform.integrations.apple.write",
  PlatformAuthSettingsRead: "platform.auth.settings.read",
  PlatformAuthSettingsWrite: "platform.auth.settings.write",
  PlatformPlansRead: "platform.plans.read",
  PlatformPlansWrite: "platform.plans.write",
  PlatformSubscriptionsRead: "platform.subscriptions.read",
  PlatformSubscriptionsWrite: "platform.subscriptions.write",
  PlatformAccountsRead: "platform.accounts.read",
  PlatformAccountsWrite: "platform.accounts.write",
  PlatformPaymentRead: "platform.payment.read",
  PlatformPaymentWrite: "platform.payment.write",
  PlatformOverviewRead: "platform.overview.read",
  PlatformProfileVerificationsManage: "platform.profile_verifications.manage",
  PlatformAnalyticsRead: "platform.analytics.read",
  PlatformAnalyticsManage: "platform.analytics.manage",
  PlatformGeoIPRead: "platform.geoip.read",
  PlatformGeoIPManage: "platform.geoip.manage",
  PlatformEventsManage: "platform.events.manage",
  PlatformErrorTrackingRead: "platform.error_tracking.read",
  PlatformErrorTrackingWrite: "platform.error_tracking.write",
  PlatformSocialPlatformsRead: "platform.social_platforms.read",
  PlatformSocialPlatformsWrite: "platform.social_platforms.write",
  PlatformBanksRead: "platform.banks.read",
  PlatformBanksWrite: "platform.banks.write",
  PlatformCategoriesRead: "platform.categories.read",
  PlatformCategoriesWrite: "platform.categories.write",
  PlatformProductsRead: "platform.products.read",
  PlatformProductsWrite: "platform.products.write",
  PlatformOrdersRead: "platform.orders.read",
  PlatformOrdersWrite: "platform.orders.write",
  PlatformCardsRead: "platform.cards.read",
  PlatformCardsWrite: "platform.cards.write",
  PlatformRoutesRead: "platform.routes.read",
  PlatformRoutesWrite: "platform.routes.write",
  AppOrdersRead: "app.orders.read",
  AppOrdersWrite: "app.orders.write",
  AppCardsRead: "app.cards.read",
  AppCardsWrite: "app.cards.write",
  AppRoutesRead: "app.routes.read",
  AppRoutesWrite: "app.routes.write",
  AppProfilesRead: "app.profiles.read",
  AppProfilesWrite: "app.profiles.write",
  AppBankAccountsRead: "app.bank_accounts.read",
  AppBankAccountsWrite: "app.bank_accounts.write",
  AppLeadsRead: "app.leads.read",
  AppLeadsWrite: "app.leads.write",
  AppWebhooksRead: "app.webhooks.read",
  AppWebhooksWrite: "app.webhooks.write",

  AuthSession: "auth.session",

  NotificationsRead: "notifications.read",
  NotificationsManage: "notifications.manage",
} as const;

export type PermissionSlug = (typeof Permission)[keyof typeof Permission];

export const ALL_PERMISSIONS: readonly PermissionSlug[] = Object.values(
  Permission,
).sort((a, b) => a.localeCompare(b));

const knownPermissionSet = new Set<string>(ALL_PERMISSIONS);

export function isKnownPermission(slug: string): slug is PermissionSlug {
  return knownPermissionSet.has(slug);
}

export const permissions = {
  users: {
    read: Permission.PlatformUsersRead,
    write: Permission.PlatformUsersWrite,
    export: Permission.PlatformUsersExport,
    import: Permission.PlatformUsersImport,
    bulkDisable: Permission.PlatformUsersBulkDisable,
    bulkEnable: Permission.PlatformUsersBulkEnable,
    impersonate: Permission.PlatformUsersImpersonate,
  },
  roles: {
    read: Permission.PlatformRolesRead,
    write: Permission.PlatformRolesWrite,
    export: Permission.PlatformRolesExport,
    import: Permission.PlatformRolesImport,
    bulkDelete: Permission.PlatformRolesBulkDelete,
  },
  notifications: {
    read: Permission.NotificationsRead,
    manage: Permission.NotificationsManage,
    platformRead: Permission.PlatformNotificationsRead,
    platformReadAll: Permission.PlatformNotificationsReadAll,
    export: Permission.PlatformNotificationsExport,
  },
  settings: {
    read: Permission.PlatformSettingsRead,
    write: Permission.PlatformSettingsWrite,
  },
  activity: {
    read: Permission.PlatformActivityRead,
  },
  imports: {
    read: Permission.PlatformImportsRead,
  },
  exports: {
    read: Permission.PlatformExportsRead,
  },
  storage: {
    read: Permission.PlatformStorageRead,
    write: Permission.PlatformStorageWrite,
  },
  logs: {
    read: Permission.PlatformLogsRead,
    write: Permission.PlatformLogsWrite,
  },
  bulk: {
    read: Permission.PlatformBulkRead,
  },
  access: {
    read: Permission.PlatformAccessRead,
    write: Permission.PlatformAccessWrite,
  },
  integrations: {
    github: {
      read: Permission.PlatformIntegrationsGitHubRead,
      write: Permission.PlatformIntegrationsGitHubWrite,
    },
    google: {
      read: Permission.PlatformIntegrationsGoogleRead,
      write: Permission.PlatformIntegrationsGoogleWrite,
    },
    facebook: {
      read: Permission.PlatformIntegrationsFacebookRead,
      write: Permission.PlatformIntegrationsFacebookWrite,
    },
    apple: {
      read: Permission.PlatformIntegrationsAppleRead,
      write: Permission.PlatformIntegrationsAppleWrite,
    },
  },
  authSettings: {
    read: Permission.PlatformAuthSettingsRead,
    write: Permission.PlatformAuthSettingsWrite,
  },
  plans: {
    read: Permission.PlatformPlansRead,
    write: Permission.PlatformPlansWrite,
  },
  subscriptions: {
    read: Permission.PlatformSubscriptionsRead,
    write: Permission.PlatformSubscriptionsWrite,
  },
  // Company accounts are managed through /v1/platform/accounts.
  companies: {
    read: Permission.PlatformAccountsRead,
    write: Permission.PlatformAccountsWrite,
  },
  payment: {
    read: Permission.PlatformPaymentRead,
    write: Permission.PlatformPaymentWrite,
  },
  overview: {
    read: Permission.PlatformOverviewRead,
  },
  profileVerifications: {
    manage: Permission.PlatformProfileVerificationsManage,
  },
  analytics: {
    read: Permission.PlatformAnalyticsRead,
    manage: Permission.PlatformAnalyticsManage,
  },
  geoip: {
    read: Permission.PlatformGeoIPRead,
    manage: Permission.PlatformGeoIPManage,
  },
  events: {
    manage: Permission.PlatformEventsManage,
  },
  errorTracking: {
    read: Permission.PlatformErrorTrackingRead,
    write: Permission.PlatformErrorTrackingWrite,
  },
  socialPlatforms: {
    read: Permission.PlatformSocialPlatformsRead,
    write: Permission.PlatformSocialPlatformsWrite,
  },
  sectors: {
    read: Permission.PlatformSectorsRead,
    write: Permission.PlatformSectorsWrite,
  },
  companyStructure: {
    read: Permission.AppCompanyStructureRead,
    write: Permission.AppCompanyStructureWrite,
  },
  companyProfiles: {
    bulk: Permission.AppCompanyProfilesBulk,
    import: Permission.AppCompanyProfilesImport,
  },
  companyAnalytics: {
    read: Permission.AppCompanyAnalyticsRead,
  },
  companyBranding: {
    read: Permission.AppCompanyBrandingRead,
    write: Permission.AppCompanyBrandingWrite,
  },
  banks: {
    read: Permission.PlatformBanksRead,
    write: Permission.PlatformBanksWrite,
  },
  categories: {
    read: Permission.PlatformCategoriesRead,
    write: Permission.PlatformCategoriesWrite,
  },
  products: {
    read: Permission.PlatformProductsRead,
    write: Permission.PlatformProductsWrite,
  },
  orders: {
    read: Permission.PlatformOrdersRead,
    write: Permission.PlatformOrdersWrite,
    appRead: Permission.AppOrdersRead,
    appWrite: Permission.AppOrdersWrite,
  },
  cards: {
    read: Permission.PlatformCardsRead,
    write: Permission.PlatformCardsWrite,
    appRead: Permission.AppCardsRead,
    appWrite: Permission.AppCardsWrite,
  },
  routes: {
    read: Permission.PlatformRoutesRead,
    write: Permission.PlatformRoutesWrite,
    appRead: Permission.AppRoutesRead,
    appWrite: Permission.AppRoutesWrite,
  },
  platformCategoriesRead: Permission.PlatformCategoriesRead,
  platformCategoriesWrite: Permission.PlatformCategoriesWrite,
  platformProductsRead: Permission.PlatformProductsRead,
  platformProductsWrite: Permission.PlatformProductsWrite,
  platformOrdersRead: Permission.PlatformOrdersRead,
  platformOrdersWrite: Permission.PlatformOrdersWrite,
  appOrdersRead: Permission.AppOrdersRead,
  appOrdersWrite: Permission.AppOrdersWrite,
  profiles: {
    read: Permission.AppProfilesRead,
    write: Permission.AppProfilesWrite,
  },
  bankAccounts: {
    read: Permission.AppBankAccountsRead,
    write: Permission.AppBankAccountsWrite,
  },
  leads: {
    read: Permission.AppLeadsRead,
    write: Permission.AppLeadsWrite,
  },
  webhooks: {
    read: Permission.AppWebhooksRead,
    write: Permission.AppWebhooksWrite,
  },
  auth: {
    session: Permission.AuthSession,
  },
} as const;
