import { useQuery } from "@tanstack/react-query";
import { client, API_ROUTES, APIRes } from "@/lib/api";
import { NotificationsOverviewResult } from "./notification_types";

export function useNotificationsOverview(projectId: string) {
    return useQuery({
        queryKey: ["notifications-overview", projectId],
        queryFn: async () => {
            const res = await client.get<APIRes<NotificationsOverviewResult>>(
                API_ROUTES.project.notifications.overview(projectId)
            );
            return res.data.data;
        },
        enabled: !!projectId,
    });
}
