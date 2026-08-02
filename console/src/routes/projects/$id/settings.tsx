import { createFileRoute } from "@tanstack/react-router";

import { ProjectSettings } from "@/features/project/settings/project_settings";
import {
    DEFAULT_SETTINGS_TAB,
    SETTINGS_TABS,
} from "@/features/project/project_types";
import { validateViewSearch } from "@/lib/search";

export const Route = createFileRoute("/projects/$id/settings")({
    // Which tab is open lives in the URL so a refresh doesn't drop you back on
    // targeting when you were part-way through configuring email.
    validateSearch: validateViewSearch(
        "tab",
        SETTINGS_TABS,
        DEFAULT_SETTINGS_TAB
    ),
    component: RouteComponent,
});

function RouteComponent() {
    const { tab } = Route.useSearch();
    const navigate = Route.useNavigate();

    return (
        <ProjectSettings
            tab={tab}
            // `replace` so switching tabs doesn't pile up history entries.
            onTabChange={(tab) => navigate({ search: { tab }, replace: true })}
        />
    );
}
