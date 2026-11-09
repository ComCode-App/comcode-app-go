"use client";

import { Contact } from "lucide-react";

import { permissions } from "@/config/permissions";
import { routes } from "@/config/routes";
import { defineNavItem } from "@/features/nav-engine";

export const profilesNavItem = defineNavItem({
  id: "profiles",
  titleKey: "layout.nav_profiles",
  href: routes.app.profiles.root,
  icon: Contact,
  permission: permissions.profiles.read,
});
