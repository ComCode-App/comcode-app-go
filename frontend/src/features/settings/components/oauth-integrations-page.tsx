"use client";

import { useState } from "react";
import { appleNavIcon } from "@/components/icons/apple-icon";
import { facebookNavIcon } from "@/components/icons/facebook-icon";
import { githubNavIcon } from "@/components/icons/github-icon";
import { googleNavIcon } from "@/components/icons/google-icon";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { permissions } from "@/config/permissions";
import { GitHubIntegrationSettingsPage } from "@/features/integrations/github/components/github-integration-settings-page";
import { OAuthProviderSettingsPage } from "@/features/integrations/oauth/components/oauth-provider-settings-page";
import { useLocale } from "@/providers/locale-provider";

export function OAuthIntegrationsPage() {
  const { t } = useLocale();
  const [activeTab, setActiveTab] = useState("google");

  const GoogleIcon = googleNavIcon;
  const GitHubIcon = githubNavIcon;
  const FacebookIcon = facebookNavIcon;
  const AppleIcon = appleNavIcon;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-bold tracking-tight text-foreground md:text-2xl">
          {t("settings.subnav.oauth_providers")}
        </h1>
        <p className="text-sm text-muted-foreground mt-1">
          {t("integrations.description")}
        </p>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <TabsList className="grid w-full grid-cols-4 max-w-xl">
          <TabsTrigger value="google" className="flex items-center gap-2 text-xs">
            <GoogleIcon className="size-3.5" />
            Google
          </TabsTrigger>
          <TabsTrigger value="github" className="flex items-center gap-2 text-xs">
            <GitHubIcon className="size-3.5" />
            GitHub
          </TabsTrigger>
          <TabsTrigger value="facebook" className="flex items-center gap-2 text-xs">
            <FacebookIcon className="size-3.5" />
            Facebook
          </TabsTrigger>
          <TabsTrigger value="apple" className="flex items-center gap-2 text-xs">
            <AppleIcon className="size-3.5" />
            Apple
          </TabsTrigger>
        </TabsList>

        <TabsContent value="google" className="mt-4">
          <OAuthProviderSettingsPage
            provider="google"
            writePermission={permissions.integrations.google.write}
          />
        </TabsContent>

        <TabsContent value="github" className="mt-4">
          <GitHubIntegrationSettingsPage />
        </TabsContent>

        <TabsContent value="facebook" className="mt-4">
          <OAuthProviderSettingsPage
            provider="facebook"
            writePermission={permissions.integrations.facebook.write}
          />
        </TabsContent>

        <TabsContent value="apple" className="mt-4">
          <OAuthProviderSettingsPage
            provider="apple"
            writePermission={permissions.integrations.apple.write}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
