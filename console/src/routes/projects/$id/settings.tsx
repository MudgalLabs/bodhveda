import { createFileRoute } from "@tanstack/react-router";
import { useDocumentTitle } from "netra";

import { EmailSettings } from "@/features/email_settings/email_settings";
import { StrictTargets } from "@/features/project/settings/strict_targets";

// Project settings, in sections. This route was the email page; it now also
// carries project-wide send behaviour, so the sections stack rather than the
// page belonging to any one of them.
function ProjectSettings() {
    useDocumentTitle("Settings • Bodhveda");

    return (
        <div>
            <StrictTargets />
            <EmailSettings />
        </div>
    );
}

export const Route = createFileRoute("/projects/$id/settings")({
    component: ProjectSettings,
});
