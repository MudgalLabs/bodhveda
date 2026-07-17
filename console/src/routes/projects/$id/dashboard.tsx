import { createFileRoute } from "@tanstack/react-router";

import { validateAnalyticsSearch } from "@/features/dashboard/analytics_range";
import { Dashboard } from "@/features/dashboard/dashboard";

export const Route = createFileRoute("/projects/$id/dashboard")({
    // The analytics date range lives in the URL so a Dashboard view is shareable
    // and survives a reload — the route owns that state and passes value +
    // onChange down, the same convention the notifications filters use (Phase 9.4).
    validateSearch: validateAnalyticsSearch,
    component: RouteComponent,
});

function RouteComponent() {
    const range = Route.useSearch();
    const navigate = Route.useNavigate();

    return (
        <Dashboard
            range={range}
            // `replace` so nudging the range doesn't pile up a history entry per
            // click for the back button to chew through.
            onRangeChange={(next) =>
                navigate({ search: next, replace: true })
            }
        />
    );
}
