import { registerCatalogs, type MessageCatalogs } from "@/lib/i18n/messages";

import enAccess from "@/locales/en/access.json";
import enActivity from "@/locales/en/activity.json";
import enBilling from "@/locales/en/billing.json";
import enBranding from "@/locales/en/branding.json";
import enBulk from "@/locales/en/bulk.json";
import enChart from "@/locales/en/chart.json";
import enApp from "@/locales/en/app.json";
import enDashboard from "@/locales/en/dashboard.json";
import enEditor from "@/locales/en/editor.json";
import enEntity from "@/locales/en/entity.json";
import enExports from "@/locales/en/exports.json";
import enGeoip from "@/locales/en/geoip.json";
import enImports from "@/locales/en/imports.json";
import enIntegrations from "@/locales/en/integrations.json";
import enLogs from "@/locales/en/logs.json";
import enLayout from "@/locales/en/layout.json";
import enNotifications from "@/locales/en/notifications.json";
import enAccounts from "@/locales/en/accounts.json";
import enSectors from "@/locales/en/sectors.json";
import enCompanyStructure from "@/locales/en/company_structure.json";
import enCompanyBranding from "@/locales/en/company_branding.json";
import enCompanyBulk from "@/locales/en/company_bulk.json";
import enSmartLinks from "@/locales/en/smart_links.json";
import enAnalytics from "@/locales/en/analytics.json";
import enPlatformAnalytics from "@/locales/en/platform_analytics.json";
import enCompanies from "@/locales/en/companies.json";
import enPermissions from "@/locales/en/permissions.json";
import enPlans from "@/locales/en/plans.json";
import enProfiles from "@/locales/en/profiles.json";
import enSocialPlatforms from "@/locales/en/social_platforms.json";
import enBanks from "@/locales/en/banks.json";
import enBankAccounts from "@/locales/en/bank_accounts.json";
import enWebhooks from "@/locales/en/webhooks.json";
import enBackgroundStudio from "@/locales/en/background_studio.json";
import enCategories from "@/locales/en/categories.json";
import enProducts from "@/locales/en/products.json";
import enOrders from "@/locales/en/orders.json";
import enRoutes from "@/locales/en/routes.json";
import enSubscriptions from "@/locales/en/subscriptions.json";
import enRoles from "@/locales/en/roles.json";
import enSearch from "@/locales/en/search.json";
import enRealtime from "@/locales/en/realtime.json";
import enSettings from "@/locales/en/settings.json";
import enStepup from "@/locales/en/stepup.json";
import enStorage from "@/locales/en/storage.json";
import enTable from "@/locales/en/table.json";
import enUsers from "@/locales/en/users.json";
import trAccess from "@/locales/tr/access.json";
import trActivity from "@/locales/tr/activity.json";
import trBilling from "@/locales/tr/billing.json";
import trBranding from "@/locales/tr/branding.json";
import trBulk from "@/locales/tr/bulk.json";
import trChart from "@/locales/tr/chart.json";
import trApp from "@/locales/tr/app.json";
import trDashboard from "@/locales/tr/dashboard.json";
import trEditor from "@/locales/tr/editor.json";
import trEntity from "@/locales/tr/entity.json";
import trExports from "@/locales/tr/exports.json";
import trGeoip from "@/locales/tr/geoip.json";
import trImports from "@/locales/tr/imports.json";
import trIntegrations from "@/locales/tr/integrations.json";
import trLogs from "@/locales/tr/logs.json";
import trLayout from "@/locales/tr/layout.json";
import trNotifications from "@/locales/tr/notifications.json";
import trAccounts from "@/locales/tr/accounts.json";
import trSectors from "@/locales/tr/sectors.json";
import trCompanyStructure from "@/locales/tr/company_structure.json";
import trCompanyBranding from "@/locales/tr/company_branding.json";
import trCompanyBulk from "@/locales/tr/company_bulk.json";
import trSmartLinks from "@/locales/tr/smart_links.json";
import trAnalytics from "@/locales/tr/analytics.json";
import trPlatformAnalytics from "@/locales/tr/platform_analytics.json";
import trCompanies from "@/locales/tr/companies.json";
import trPermissions from "@/locales/tr/permissions.json";
import trPlans from "@/locales/tr/plans.json";
import trProfiles from "@/locales/tr/profiles.json";
import trSocialPlatforms from "@/locales/tr/social_platforms.json";
import trBanks from "@/locales/tr/banks.json";
import trBankAccounts from "@/locales/tr/bank_accounts.json";
import trWebhooks from "@/locales/tr/webhooks.json";
import trBackgroundStudio from "@/locales/tr/background_studio.json";
import trCategories from "@/locales/tr/categories.json";
import trProducts from "@/locales/tr/products.json";
import trOrders from "@/locales/tr/orders.json";
import trRoutes from "@/locales/tr/routes.json";
import trSubscriptions from "@/locales/tr/subscriptions.json";
import trRoles from "@/locales/tr/roles.json";
import trSearch from "@/locales/tr/search.json";
import trRealtime from "@/locales/tr/realtime.json";
import trSettings from "@/locales/tr/settings.json";
import trStepup from "@/locales/tr/stepup.json";
import trStorage from "@/locales/tr/storage.json";
import trTable from "@/locales/tr/table.json";
import trUsers from "@/locales/tr/users.json";

/**
 * Panel namespaces (app, platform, guest). Imported for its side effect by the (shell) providers so
 * visitor pages ship only the public namespaces in `messages.ts` (TEC-858).
 */
export const appCatalogs: MessageCatalogs = {
  tr: {
    layout: trLayout,
    editor: trEditor,
    table: trTable,
    chart: trChart,
    branding: trBranding,
    bulk: trBulk,
    entity: trEntity,
    permissions: trPermissions,
    realtime: trRealtime,
    app: trApp,
    billing: trBilling,
    plans: trPlans,
    profiles: trProfiles,
    social_platforms: trSocialPlatforms,
    banks: trBanks,
    bank_accounts: trBankAccounts,
    webhooks: trWebhooks,
    background_studio: trBackgroundStudio,
    categories: trCategories,
    products: trProducts,
    orders: trOrders,
    routes: trRoutes,
    subscriptions: trSubscriptions,
    accounts: trAccounts,
    sectors: trSectors,
    company_structure: trCompanyStructure,
    company_branding: trCompanyBranding,
    company_bulk: trCompanyBulk,
    smart_links: trSmartLinks,
    analytics: trAnalytics,
    platform_analytics: trPlatformAnalytics,
    companies: trCompanies,
    roles: trRoles,
    search: trSearch,
    users: trUsers,
    notifications: trNotifications,
    dashboard: trDashboard,
    activity: trActivity,
    exports: trExports,
    imports: trImports,
    logs: trLogs,
    settings: trSettings,
    storage: trStorage,
    geoip: trGeoip,
    access: trAccess,
    integrations: trIntegrations,
    stepup: trStepup,
  },
  en: {
    layout: enLayout,
    editor: enEditor,
    table: enTable,
    chart: enChart,
    branding: enBranding,
    bulk: enBulk,
    entity: enEntity,
    permissions: enPermissions,
    realtime: enRealtime,
    app: enApp,
    billing: enBilling,
    plans: enPlans,
    profiles: enProfiles,
    social_platforms: enSocialPlatforms,
    banks: enBanks,
    bank_accounts: enBankAccounts,
    webhooks: enWebhooks,
    background_studio: enBackgroundStudio,
    categories: enCategories,
    products: enProducts,
    orders: enOrders,
    routes: enRoutes,
    subscriptions: enSubscriptions,
    accounts: enAccounts,
    sectors: enSectors,
    company_structure: enCompanyStructure,
    company_branding: enCompanyBranding,
    company_bulk: enCompanyBulk,
    smart_links: enSmartLinks,
    analytics: enAnalytics,
    platform_analytics: enPlatformAnalytics,
    companies: enCompanies,
    roles: enRoles,
    search: enSearch,
    users: enUsers,
    notifications: enNotifications,
    dashboard: enDashboard,
    activity: enActivity,
    exports: enExports,
    imports: enImports,
    logs: enLogs,
    settings: enSettings,
    storage: enStorage,
    geoip: enGeoip,
    access: enAccess,
    integrations: enIntegrations,
    stepup: enStepup,
  },
};

registerCatalogs(appCatalogs);
