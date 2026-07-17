import { keepPreviousData, useQuery } from "@tanstack/react-query";

import {
    AnalyticsRange,
    rangeToParams,
    viewerTimezone,
} from "@/features/dashboard/analytics_range";
import { ProjectAnalytics } from "@/features/dashboard/analytics_types";
import { API_ROUTES, APIRes, client } from "@/lib/api";

// useProjectAnalytics fetches the Home page's ranged analytics (Phase 9.5).
//
// The viewer's timezone rides the X-Timezone header (not a query param): the API
// buckets per-day in it via TimezoneMiddleware, so "a day" is the operator's day.
// The resolved absolute range is part of the query key so switching preset/range
// refetches; a preset is relative, so the key also carries whether it is custom.
export function useProjectAnalytics(projectID: string, range: AnalyticsRange) {
    const params = rangeToParams(range);
    const tz = viewerTimezone();

    return useQuery({
        queryKey: [
            "useProjectAnalytics",
            projectID,
            params.created_from,
            params.created_to,
        ],
        queryFn: () =>
            client.get(API_ROUTES.project.analytics(projectID), {
                params,
                headers: { "X-Timezone": tz },
            }),
        select: (res) => res.data as APIRes<ProjectAnalytics>,
        enabled: !!projectID,
        // Hold the previous range's charts while the new one loads, so switching
        // preset doesn't flash an empty dashboard.
        placeholderData: keepPreviousData,
    });
}
