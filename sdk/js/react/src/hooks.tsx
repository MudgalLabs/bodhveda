import { useContext } from "react";
import {
    AnyUseMutationOptions,
    AnyUseQueryOptions,
    useInfiniteQuery,
    UseInfiniteQueryOptions,
    UseInfiniteQueryResult,
    useMutation,
    useQuery,
    useQueryClient,
    UseQueryResult,
} from "@tanstack/react-query";
import {
    ListNotificationsResponse,
    ListPreferencesResponse,
    UpdateNotificationsStateRequest,
    UpdateNotificationsStateResponse,
    SetPreferenceRequest,
    SetPreferenceResponse,
    DeleteNotificationsResponse,
    DeleteNotificationsRequest,
    Target,
    CheckPreferenceResponse,
    ListNotificationsRequest,
} from "bodhveda";

import { BodhvedaContext } from "./context";

type QueryOptions = Omit<AnyUseQueryOptions, "queryKey">;
type MutationOptions = AnyUseMutationOptions;

export const QueryKeys = {
    useNotifications: ["useNotifications"],
    useNotificationsUnreadCount: ["useNotificationsUnreadCount"],
    useCheckPreference: ["useCheckPreference"],
    usePreferences: ["usePreferences"],
};

/**
 * Returns the Bodhveda client instance.
 *
 * @throws If used outside a {@link BodhvedaProvider}.
 * @returns {Bodhveda} The Bodhveda client instance.
 */
export function useBodhveda() {
    const context = useContext(BodhvedaContext);

    if (!context) {
        throw new Error("useBodhveda: did you forget to use BodhvedaProvider?");
    }

    return context.bodhveda;
}

/**
 * Returns the current recipient ID.
 *
 * @throws If used outside a {@link BodhvedaProvider}.
 * @returns {string} The recipient ID.
 */
export function useRecipientID() {
    const context = useContext(BodhvedaContext);

    if (!context) {
        throw new Error("useBodhveda: did you forget to use BodhvedaProvider?");
    }

    return context.recipientID;
}

/**
 * Fetches the list of notifications for the current recipient with infinite scrolling support.
 *
 * @param req - Optional request parameters for listing notifications.
 * @param options - Optional react-query infinite query options.
 * @returns {UseInfiniteQueryResult<ListNotificationsResponse, Error>} Infinite query result containing notifications and pagination info.
 */
export function useNotifications(
    req: ListNotificationsRequest = {},
    options: Partial<
        UseInfiniteQueryOptions<
            ListNotificationsResponse,
            Error,
            ListNotificationsResponse,
            [typeof QueryKeys.useNotifications]
        >
    > = {}
): UseInfiniteQueryResult<ListNotificationsResponse, Error> {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const { limit = 10 } = req;

    return useInfiniteQuery<
        ListNotificationsResponse,
        Error,
        ListNotificationsResponse,
        [typeof QueryKeys.useNotifications]
    >({
        queryKey: [QueryKeys.useNotifications],
        queryFn: ({ pageParam }) => {
            return bodhveda.recipients.notifications.list(recipientID, {
                limit,
                before: pageParam ? String(pageParam) : undefined,
            });
        },
        initialPageParam: null,
        getNextPageParam: (lastPage) => lastPage.cursor.before,
        select: (data) => {
            return {
                ...data.pages[data.pages.length - 1],
                notifications: data.pages.flatMap((page) => page.notifications),
            };
        },
        ...options,
    });
}

/**
 * Fetches the unread notifications count for the current recipient.
 *
 * @param options - Optional react-query options.
 * @returns {UseQueryResult<{ unread_count: number }>} Query result.
 */
export function useNotificationsUnreadCount(
    options: QueryOptions = {}
): UseQueryResult<{ unread_count: number }> {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();

    return useQuery({
        queryKey: [QueryKeys.useNotificationsUnreadCount],
        queryFn: () =>
            bodhveda.recipients.notifications.unreadCount(recipientID),
        ...options,
    });
}

/**
 * Returns a mutation hook to update notification state (e.g., mark as read).
 *
 * @param options - Optional mutation options.
 * @returns Mutation hook.
 */
export function useUpdateNotificationsState(options: MutationOptions = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const queryClient = useQueryClient();
    const { onSuccess, ...rest } = options;

    return useMutation<
        UpdateNotificationsStateResponse,
        unknown,
        UpdateNotificationsStateRequest,
        unknown
    >({
        mutationFn: (req) => {
            return bodhveda.recipients.notifications.updateState(
                recipientID,
                req
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: [QueryKeys.useNotifications],
            });
            queryClient.invalidateQueries({
                queryKey: [QueryKeys.useNotificationsUnreadCount],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

/**
 * Returns a mutation hook to delete notifications for the current recipient.
 *
 * @param options - Optional mutation options.
 * @returns Mutation hook.
 */
export function useDeleteNotifications(options: MutationOptions = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const queryClient = useQueryClient();
    const { onSuccess, ...rest } = options;

    return useMutation<
        DeleteNotificationsResponse,
        unknown,
        DeleteNotificationsRequest,
        unknown
    >({
        mutationFn: (req) => {
            return bodhveda.recipients.notifications.delete(recipientID, req);
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: [QueryKeys.useNotifications],
            });
            queryClient.invalidateQueries({
                queryKey: [QueryKeys.useNotificationsUnreadCount],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

/**
 * Fetches the notification preferences for the current recipient.
 *
 * @param options - Optional react-query options.
 * @returns {UseQueryResult<ListPreferencesResponse>} Query result.
 */
export function usePreferences(
    options: QueryOptions = {}
): UseQueryResult<ListPreferencesResponse> {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();

    return useQuery({
        queryKey: [QueryKeys.usePreferences],
        queryFn: () => bodhveda.recipients.preferences.list(recipientID),
        ...options,
    });
}

/**
 * Returns a mutation hook to update a notification preference for the current recipient.
 *
 * @param options - Optional mutation options.
 * @returns Mutation hook.
 */
export function useUpdatePreference(options: MutationOptions = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const queryClient = useQueryClient();
    const { onSuccess, ...rest } = options;

    return useMutation<
        SetPreferenceResponse,
        unknown,
        SetPreferenceRequest,
        unknown
    >({
        mutationFn: (req: SetPreferenceRequest) => {
            return bodhveda.recipients.preferences.set(recipientID, req);
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: [QueryKeys.usePreferences],
            });
            queryClient.invalidateQueries({
                queryKey: [QueryKeys.useCheckPreference, args[0].target],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

/**
 * Checks a specific notification preference for the current recipient.
 *
 * @param target - The notification target/channel to check.
 * @param options - Optional react-query options.
 * @returns {UseQueryResult<CheckPreferenceResponse>} Query result.
 */
export function useCheckPreference(
    target: Target,
    options: QueryOptions = {}
): UseQueryResult<CheckPreferenceResponse> {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();

    return useQuery({
        queryKey: [QueryKeys.useCheckPreference, target],
        queryFn: () =>
            bodhveda.recipients.preferences.check(recipientID, { target }),
        ...options,
    });
}
