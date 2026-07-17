import { createFileRoute } from "@tanstack/react-router";

import { ProjectPreferenceList } from "@/features/preference/list/preference_list";
import {
    DEFAULT_PREFERENCE_KIND,
    PREFERENCE_KINDS,
} from "@/features/preference/preference_type";
import { validateViewSearch } from "@/lib/search";

export const Route = createFileRoute("/projects/$id/preferences")({
    // Which kind is being viewed lives in the URL so a refresh doesn't drop you
    // back on the project catalog when you were reading recipient preferences.
    validateSearch: validateViewSearch(
        "kind",
        PREFERENCE_KINDS,
        DEFAULT_PREFERENCE_KIND
    ),
    component: RouteComponent,
});

function RouteComponent() {
    const { kind } = Route.useSearch();
    const navigate = Route.useNavigate();

    return (
        <ProjectPreferenceList
            kind={kind}
            // `replace` so flipping the toggle doesn't pile up history entries.
            onKindChange={(kind) =>
                navigate({ search: { kind }, replace: true })
            }
        />
    );
}
