import { createFileRoute } from "@tanstack/react-router";

import { EmailSettings } from "@/features/email_settings/email_settings";

export const Route = createFileRoute("/projects/$id/settings")({
    component: EmailSettings,
});
