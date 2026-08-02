import {
    IconSettings,
    PageHeading,
    ToggleGroup,
    ToggleGroupItem,
    useDocumentTitle,
} from "netra";

import { EmailSettings } from "@/features/email_settings/email_settings";
import { TargetingSettings } from "@/features/project/settings/targeting_settings";
import { SettingsTab } from "@/features/project/project_types";

interface ProjectSettingsProps {
    /** The tab being viewed. Owned by the route, which reads it from the URL. */
    tab: SettingsTab;
    onTabChange: (tab: SettingsTab) => void;
}

/**
 * Project settings, one tab per subject.
 *
 * Tabs rather than stacked sections because a page owns exactly one
 * PageHeading — two of them rendered two sidebar-collapse controls and read as
 * two pages that had been glued together. The Preferences page already switches
 * between two views this way, so the pattern is the app's, not a new one.
 */
export function ProjectSettings({ tab, onTabChange }: ProjectSettingsProps) {
    useDocumentTitle("Settings • Bodhveda");

    return (
        <div>
            <PageHeading>
                <IconSettings size={18} />
                <h1>Settings</h1>
            </PageHeading>

            <div className="flex justify-between mb-4">
                <ToggleGroup
                    className="[&_*]:h-8 pl-0!"
                    type="single"
                    size="small"
                    value={tab}
                    onValueChange={(value) =>
                        value && onTabChange(value as SettingsTab)
                    }
                >
                    <ToggleGroupItem value="targeting">
                        Targeting
                    </ToggleGroupItem>
                    <ToggleGroupItem value="email">Email</ToggleGroupItem>
                </ToggleGroup>
            </div>

            {tab === "targeting" ? <TargetingSettings /> : <EmailSettings />}
        </div>
    );
}
