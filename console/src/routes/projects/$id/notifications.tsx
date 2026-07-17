import { createFileRoute } from "@tanstack/react-router";

import { NotificationList } from "@/features/notification/list/notifications_list";
import { validateNotificationSearch } from "@/features/notification/notification_filters";

export const Route = createFileRoute("/projects/$id/notifications")({
    // The whole filter selection — which kind is being viewed included — lives in
    // the URL, so a refresh doesn't drop you back on an unfiltered `direct` list
    // and a filtered view can be shared as a link.
    validateSearch: validateNotificationSearch,
    component: RouteComponent,
});

function RouteComponent() {
    const filters = Route.useSearch();
    const navigate = Route.useNavigate();

    return (
        <NotificationList
            filters={filters}
            // `replace` so refining a filter doesn't pile up one history entry
            // per tweak for the back button to chew through.
            onFiltersChange={(filters) =>
                navigate({ search: filters, replace: true })
            }
        />
    );
}
