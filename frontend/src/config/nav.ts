import {
  Activity,
  BadgeCheck,
  BarChart3,
  Bell,
  Box,
  Bug,
  Building2,
  Cpu,
  CreditCard,
  Download,
  Factory,
  Globe,
  HardDrive,
  Inbox,
  Kanban,
  KeyRound,
  Landmark,
  Layers,
  LayoutDashboard,
  Link2,
  ListTree,
  LogIn,
  Palette,
  ScrollText,
  Settings2,
  Share2,
  Shield,
  ShoppingBag,
  SlidersHorizontal,
  Sparkles,
  TableProperties,
  Upload,
  UsersRound,
  Webhook,
} from "lucide-react";

import { paytrNavIcon } from "@/components/icons/paytr-icon";
import { iyzicoNavIcon } from "@/components/icons/iyzico-icon";
import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { defineNav } from "@/features/nav-engine";
import { backgroundStudioNavItem } from "@/features/background-studio/nav";
import { companiesNavItem } from "@/features/companies/nav";
import { companyStructureNavGroup } from "@/features/company-structure/nav";
import { profilesNavItem } from "@/features/profiles/nav";
import { usersNavItem } from "@/features/users/nav";

export const platformNav = defineNav({
  id: "platform",
  groups: [
    {
      id: "general",
      labelKey: "layout.nav_platform",
      icon: LayoutDashboard,
      defaultOpen: true,
      items: [
        {
          id: "home",
          titleKey: "layout.home",
          href: routes.platform.home,
          icon: LayoutDashboard,
        },
        {
          id: "settings",
          titleKey: "layout.nav_settings",
          href: routes.platform.settings.root,
          icon: Settings2,
          permission: permissions.settings.read,
        },
      ],
    },
    {
      id: "manage",
      labelKey: "layout.section_manage",
      icon: Shield,
      defaultOpen: true,
      items: [
        usersNavItem,
        companiesNavItem,
        {
          id: "profile-verifications",
          titleKey: "layout.nav_profile_verifications",
          href: routes.platform.profileVerifications,
          icon: BadgeCheck,
          permission: permissions.profileVerifications.manage,
        },
        {
          id: "notifications",
          titleKey: "layout.nav_notifications",
          href: routes.platform.notifications.root,
          icon: Bell,
          permission: permissions.notifications.platformRead,
        },
      ],
    },
    {
      id: "analytics",
      labelKey: "layout.section_analytics",
      icon: BarChart3,
      defaultOpen: true,
      items: [
        {
          id: "platform-analytics",
          titleKey: "layout.nav_platform_analytics_overview",
          href: routes.platform.analytics.root,
          icon: BarChart3,
          permission: permissions.analytics.read,
        },
        {
          id: "platform-analytics-events",
          titleKey: "layout.nav_platform_analytics_events",
          href: routes.platform.analytics.events,
          icon: TableProperties,
          permission: permissions.analytics.read,
        },
        {
          id: "platform-event-definitions",
          titleKey: "layout.nav_platform_event_definitions",
          href: routes.platform.analytics.definitions,
          icon: ListTree,
          permission: permissions.events.manage,
        },
        {
          id: "platform-analytics-config",
          titleKey: "layout.nav_platform_analytics_config",
          href: routes.platform.analytics.config,
          icon: SlidersHorizontal,
          permission: permissions.analytics.read,
        },
      ],
    },
    {
      id: "commerce",
      labelKey: "layout.section_commerce",
      icon: ShoppingBag,
      defaultOpen: true,
      items: [
        {
          id: "orders",
          titleKey: "layout.nav_orders",
          href: routes.platform.orders,
          icon: ShoppingBag,
          permission: permissions.platformOrdersRead,
        },
        {
          id: "orders-pipeline",
          titleKey: "layout.nav_orders_pipeline",
          href: routes.platform.ordersPipeline,
          icon: Kanban,
          permission: permissions.platformOrdersRead,
        },
        {
          id: "products",
          titleKey: "layout.nav_products",
          href: routes.platform.products,
          icon: Box,
          permission: permissions.platformProductsRead,
        },
        {
          id: "categories",
          titleKey: "layout.nav_categories",
          href: routes.platform.categories,
          icon: Layers,
          permission: permissions.platformCategoriesRead,
        },
        {
          id: "card-support",
          titleKey: "layout.nav_card_support",
          href: routes.platform.support.cards,
          icon: CreditCard,
          permission: permissions.cards.read,
        },
        {
          id: "route-support",
          titleKey: "layout.nav_route_support",
          href: routes.platform.support.routes,
          icon: Link2,
          permission: permissions.routes.read,
        },
      ],
    },
  ],
});

export const settingsNav = defineNav({
  id: "settings",
  groups: [
    {
      id: "system",
      labelKey: "settings.subnav.group_system",
      icon: Activity,
      defaultOpen: true,
      items: [
        {
          id: "system-overview",
          titleKey: "settings.subnav.system_overview",
          href: routes.platform.settings.root,
          icon: Activity,
          permission: permissions.settings.read,
        },
        {
          id: "system-environment",
          titleKey: "settings.subnav.system_environment",
          href: routes.platform.settings.environment,
          icon: Cpu,
          permission: permissions.settings.read,
        },
        {
          id: "logs",
          titleKey: "layout.nav_logs",
          href: routes.platform.settings.logs,
          icon: ScrollText,
          permission: permissions.logs.read,
        },
        {
          id: "activity",
          titleKey: "layout.nav_activity",
          href: routes.platform.settings.activity,
          icon: Activity,
          permission: permissions.activity.read,
        },
      ],
    },
    {
      id: "general",
      labelKey: "settings.subnav.group_general",
      icon: Settings2,
      defaultOpen: true,
      items: [
        {
          id: "general-branding",
          titleKey: "settings.subnav.general_branding",
          href: routes.platform.settings.general,
          icon: Palette,
          permission: permissions.settings.read,
        },
        {
          id: "banks",
          titleKey: "layout.nav_banks",
          href: routes.platform.settings.banks,
          icon: Landmark,
          permission: permissions.banks.read,
        },
        {
          id: "sectors",
          titleKey: "layout.nav_sectors",
          href: routes.platform.settings.sectors,
          icon: Factory,
          permission: permissions.sectors.read,
        },
        {
          id: "social-platforms",
          titleKey: "layout.nav_social_platforms",
          href: routes.platform.settings.socialPlatforms,
          icon: Share2,
          permission: permissions.socialPlatforms.read,
        },
      ],
    },
    {
      id: "security",
      labelKey: "settings.subnav.group_security",
      icon: Shield,
      defaultOpen: true,
      items: [
        {
          id: "roles",
          titleKey: "layout.nav_roles",
          href: routes.platform.settings.roles,
          icon: Shield,
          permission: permissions.roles.read,
        },
        {
          id: "auth-settings",
          titleKey: "settings.subnav.auth_methods",
          href: routes.platform.settings.auth,
          icon: LogIn,
          permission: permissions.authSettings.read,
        },
        {
          id: "access-policy",
          titleKey: "settings.subnav.access_policy",
          href: routes.platform.settings.access,
          icon: KeyRound,
          permission: permissions.access.read,
        },
      ],
    },
    {
      id: "billing",
      labelKey: "settings.subnav.group_payment",
      icon: CreditCard,
      defaultOpen: true,
      items: [
        {
          id: "plans",
          titleKey: "layout.nav_plans",
          href: routes.platform.settings.plans,
          icon: CreditCard,
          permission: permissions.plans.read,
        },
        {
          id: "subscriptions",
          titleKey: "layout.nav_subscriptions",
          href: routes.platform.settings.subscriptions,
          icon: CreditCard,
          permission: permissions.subscriptions.read,
        },
        {
          id: "payment-settings-overview",
          titleKey: "settings.subnav.payment_overview",
          href: routes.platform.settings.payment,
          icon: Settings2,
          permission: permissions.payment.read,
        },
        {
          id: "paytr-settings",
          titleKey: "settings.subnav.payment_paytr",
          href: routes.platform.settings.paytr,
          icon: paytrNavIcon,
          permission: permissions.payment.read,
        },
        {
          id: "iyzico-settings",
          titleKey: "settings.subnav.payment_iyzico",
          href: routes.platform.settings.iyzico,
          icon: iyzicoNavIcon,
          permission: permissions.payment.read,
        },
      ],
    },
    {
      id: "integrations",
      labelKey: "settings.subnav.group_integrations",
      icon: Share2,
      defaultOpen: true,
      items: [
        {
          id: "oauth-integrations",
          titleKey: "settings.subnav.oauth_providers",
          href: routes.platform.settings.integrations,
          icon: Share2,
          permission: permissions.integrations.google.read,
        },
        {
          id: "geoip",
          titleKey: "settings.subnav.geoip",
          href: routes.platform.settings.geoip,
          icon: Globe,
          permission: permissions.geoip.read,
        },
        {
          id: "error-tracking",
          titleKey: "settings.subnav.error_tracking",
          href: routes.platform.settings.errorTracking,
          icon: Bug,
          permission: permissions.errorTracking.read,
        },
        {
          id: "storage",
          titleKey: "settings.subnav.storage",
          href: routes.platform.settings.storage,
          icon: HardDrive,
          permission: permissions.storage.read,
        },
      ],
    },
    {
      id: "io",
      labelKey: "settings.subnav.group_maintenance",
      icon: Upload,
      defaultOpen: true,
      items: [
        {
          id: "imports",
          titleKey: "layout.nav_imports",
          href: routes.platform.settings.imports,
          icon: Upload,
          permission: permissions.imports.read,
        },
        {
          id: "exports",
          titleKey: "layout.nav_exports",
          href: routes.platform.settings.exports,
          icon: Download,
          permission: permissions.exports.read,
        },
        {
          id: "purge-rules",
          titleKey: "settings.subnav.purge_rules",
          href: routes.platform.settings.maintenance,
          icon: Sparkles,
          permission: permissions.logs.read,
        },
      ],
    },
  ],
});

export const appNav = defineNav({
  id: "app",
  groups: [
    {
      id: "app",
      labelKey: "layout.section_app",
      icon: LayoutDashboard,
      defaultOpen: true,
      items: [
        {
          id: "home",
          titleKey: "layout.home",
          href: routes.app.home,
          icon: LayoutDashboard,
        },
        {
          id: "cards",
          titleKey: "layout.nav_cards",
          href: routes.app.cards.root,
          icon: CreditCard,
          permission: permissions.cards.appRead,
        },
        {
          id: "my-orders",
          titleKey: "layout.nav_my_orders",
          href: routes.app.orders.root,
          icon: ShoppingBag,
          permission: permissions.orders.appRead,
        },
        profilesNavItem,
        {
          id: "analytics",
          titleKey: "layout.nav_analytics",
          href: routes.app.analytics.root,
          icon: BarChart3,
        },
        {
          id: "smart-links",
          titleKey: "layout.nav_smart_links",
          href: routes.app.smartLinks.root,
          icon: Link2,
          permission: permissions.routes.appRead,
        },
        {
          id: "bank-accounts",
          titleKey: "layout.nav_bank_accounts",
          href: routes.app.bankAccounts.root,
          icon: Building2,
          permission: permissions.bankAccounts.read,
        },
        {
          id: "leads",
          titleKey: "layout.nav_leads",
          href: routes.app.leads.root,
          icon: Inbox,
          permission: permissions.leads.read,
        },
        {
          id: "webhooks",
          titleKey: "layout.nav_webhooks",
          href: routes.app.webhooks.root,
          icon: Webhook,
          permission: permissions.webhooks.read,
        },
        {
          id: "account",
          titleKey: "layout.nav_account",
          href: routes.app.account.root,
          icon: UsersRound,
        },
        {
          id: "billing",
          titleKey: "layout.nav_billing",
          href: routes.app.billing.root,
          icon: CreditCard,
        },
        backgroundStudioNavItem,
      ],
    },
    companyStructureNavGroup,
  ],
});

/** @deprecated Use appNav */
export const cmsNav = appNav;
