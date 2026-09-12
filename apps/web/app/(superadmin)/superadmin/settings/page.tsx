import { Settings2 } from "lucide-react";
import { PageHeader } from "@auraedu/ui";
import { requireAuth } from "@/lib/auth";
import { PlatformAccountSettings } from "@/components/platform-account-settings";

export default async function PlatformSettingsPage() {
  const session = await requireAuth();
  const user = {
    id: session.user_id ?? session.sub ?? "",
    name: session.name ?? session.email ?? "Platform administrator",
    email: session.email ?? "",
    role: session.role ?? "platform_super_admin",
  };

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<Settings2 className="size-7" />}
        title="Account & preferences"
        description="Manage your platform identity, security posture, and the visual material used throughout your console."
      />
      <PlatformAccountSettings user={user} />
    </div>
  );
}
