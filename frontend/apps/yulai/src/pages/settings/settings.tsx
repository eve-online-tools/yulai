import { EmptyState, PageHead } from "@xaroth.nl/design/react";
import { AppIcon } from "../../components/app-icon";

export function SettingsPage() {
  return (
    <>
      <PageHead title="Settings" />
      <EmptyState icon={<AppIcon name="settings" size={32} />} title="Nothing to configure yet" />
    </>
  );
}
