import {
    AnyUseMutationOptions,
    keepPreviousData,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";

import { API_ROUTES, APIRes, client } from "@/lib/api";
import { getRecipientsKey } from "@/features/recipient/recipient_hooks";
import { notificationFiltersToParams } from "@/features/notification/notification_filters";
import {
    ListBroadcastsPayload,
    ListBroadcastsResult,
    ListNotificationDeliveriesResult,
    ListNotificationsPayload,
    ListNotificationsResult,
    NotificationFilters,
    NotificationKindFilter,
    SendNotificationPayload,
    SendNotificationResult,
} from "@/features/notification/notification_types";

export function useSendNotification(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<SendNotificationResult>,
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
            queryClient.invalidateQueries({
                predicate: (query) =>
                    Array.isArray(query.queryKey) &&
                    query.queryKey[0] === "useGetNotification",
            });
            queryClient.invalidateQueries({
                predicate: (query) =>
                    Array.isArray(query.queryKey) &&
                    query.queryKey[0] === "useGetBroadcasts",
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export interface UseNotificationsParams {
    kind: NotificationKindFilter;
    page: number;
    limit: number;
    /**
     * Exact recipient external id — what the recipient detail page's feed pins
     * itself to. Distinct from `filters.recipient_search`, which is the
     * substring search on the project-wide list: one addresses a known
     * recipient, the other looks for one.
     */
    recipientID?: string;
    /** The operator's filter selection (Phase 9.4). `kind` above wins over it. */
    filters?: NotificationFilters;
}

// useNotifications lists a project's notifications. `recipientID` narrows it to
// one recipient — the recipient detail page's feed (Phase 9.2) — and `filters`
// carries the operator's URL-synced filter selection (Phase 9.4).
//
// This is the OPERATOR's view, deliberately not the recipient's inbox feed
// (`ListForRecipient` on the Developer API): it keeps `muted`/`quota_exceeded`
// rows and carries each row's email delivery outcome, which is precisely what
// someone asking "why didn't they get it?" needs to see.
export function useNotifications(
    projectID: string,
    { kind, page, limit, recipientID, filters }: UseNotificationsParams
) {
    const filterParams = filters ? notificationFiltersToParams(filters) : {};

    return useQuery({
        // filterParams (not `filters`) is the key: it is already normalized to
        // what actually goes on the wire, so two selections that request the
        // same rows share a cache entry.
        queryKey: [
            "useGetNotification",
            projectID,
            kind,
            page,
            limit,
            recipientID ?? null,
            filterParams,
        ],
        queryFn: () => {
            const params: ListNotificationsPayload = {
                kind,
                page,
                limit,
                ...(recipientID ? { recipient_id: recipientID } : {}),
                ...filterParams,
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

// useNotificationDeliveries fetches the full delivery records for ONE
// notification, including the raw provider webhook event history (Phase 9.1).
//
// `enabled` is what keeps the split honest: the history is unbounded, so it is
// fetched only when an operator actually opens the delivery detail dialog,
// rather than riding every row of every list refetch.
export function useNotificationDeliveries(
    projectID: string,
    notificationID: number,
    enabled = true
) {
    return useQuery({
        queryKey: ["useNotificationDeliveries", projectID, notificationID],
        queryFn: () =>
            client.get(
                API_ROUTES.project.notifications.deliveries(
                    projectID,
                    notificationID
                )
            ),
        select: (res) => res.data as APIRes<ListNotificationDeliveriesResult>,
        enabled: enabled && !!projectID && !!notificationID,
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
