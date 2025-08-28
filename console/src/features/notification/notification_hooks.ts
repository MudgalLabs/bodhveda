import {
    AnyUseMutationOptions,
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";

import { API_ROUTES, APIRes, client } from "@/lib/api";
import { getRecipientsKey } from "@/features/recipient/recipient_hooks";
import {
    ListBroadcastsPayload,
    ListBroadcastsResult,
    ListNotificationsPayload,
    ListNotificationsResult,
    NotificationKind,
    SendNotificationPayload,
    SendNotificationResponse,
} from "@/features/notification/notification_types";

export function useSendNotification(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<SendNotificationResponse>,
        unknown,
        SendNotificationPayload
    >({
        mutationFn: (payload) => {
            return client.post(
                API_ROUTES.project.notifications.send(projectID),
                payload
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getRecipientsKey(projectID),
            });
            queryClient.invalidateQueries({
                queryKey: ["useGetProjects"],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useNotifications(
    projectID: string,
    kind: NotificationKind,
    page: number,
    limit: number
) {
    return useQuery({
        queryKey: ["useGetNotification", projectID, kind, page, limit],
        queryFn: () => {
            const params: ListNotificationsPayload = {
                kind,
                page,
                limit,
            };

            return client.get(
                API_ROUTES.project.notifications.list(projectID),
                {
                    params,
                }
            );
        },
        select: (res) => res.data as APIRes<ListNotificationsResult>,
        placeholderData: keepPreviousData,
    });
}

export function useBroadcasts(projectID: string, page: number, limit: number) {
    return useQuery({
        queryKey: ["useGetBroadcasts", projectID, page, limit],
        queryFn: () => {
            const params: ListBroadcastsPayload = {
                page,
                limit,
            };
            return client.get(API_ROUTES.project.broadcasts.list(projectID), {
                params,
            });
        },
        select: (res) => res.data as APIRes<ListBroadcastsResult>,
        placeholderData: keepPreviousData,
    });
}
