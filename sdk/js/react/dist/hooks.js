import { useContext } from "react";
import { useMutation, useQuery, useQueryClient, } from "@tanstack/react-query";
import { BodhvedaContext } from "./context";
const QUERY_KEYS = {
    useNotifications: ["useNotifications"],
    useNotificationsUnreadCount: ["useNotificationsUnreadCount"],
    useCheckPreference: ["useCheckPreference"],
    usePreferences: ["usePreferences"],
};
export function useBodhveda() {
    const context = useContext(BodhvedaContext);
    if (!context) {
        throw new Error("useBodhveda: did you forget to use BodhvedaProvider?");
    }
    return context.bodhveda;
}
export function useRecipientID() {
    const context = useContext(BodhvedaContext);
    if (!context) {
        throw new Error("useBodhveda: did you forget to use BodhvedaProvider?");
    }
    return context.recipientID;
}
export function useNotifications(options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    return useQuery({
        queryKey: [QUERY_KEYS.useNotifications],
        queryFn: () => bodhveda.recipients.notifications.list(recipientID),
        ...options,
    });
}
export function useNotificationsUnreadCount(options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    return useQuery({
        queryKey: [QUERY_KEYS.useNotificationsUnreadCount],
        queryFn: () => bodhveda.recipients.notifications.unreadCount(recipientID),
        ...options,
    });
}
export function useUpdateNotificationsState(options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const queryClient = useQueryClient();
    const { onSuccess, ...rest } = options;
    return useMutation({
        mutationFn: (req) => {
            return bodhveda.recipients.notifications.updateState(recipientID, req);
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: [QUERY_KEYS.useNotifications],
            });
            queryClient.invalidateQueries({
                queryKey: [QUERY_KEYS.useNotificationsUnreadCount],
            });
            onSuccess === null || onSuccess === void 0 ? void 0 : onSuccess(...args);
        },
        ...rest,
    });
}
export function useDeleteNotifications(options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const queryClient = useQueryClient();
    const { onSuccess, ...rest } = options;
    return useMutation({
        mutationFn: (req) => {
            return bodhveda.recipients.notifications.delete(recipientID, req);
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: [QUERY_KEYS.useNotifications],
            });
            queryClient.invalidateQueries({
                queryKey: [QUERY_KEYS.useNotificationsUnreadCount],
            });
            onSuccess === null || onSuccess === void 0 ? void 0 : onSuccess(...args);
        },
        ...rest,
    });
}
export function usePreferences(options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    return useQuery({
        queryKey: [QUERY_KEYS.usePreferences],
        queryFn: () => bodhveda.recipients.preferences.list(recipientID),
        ...options,
    });
}
export function useUpdatePreference(options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    const queryClient = useQueryClient();
    const { onSuccess, ...rest } = options;
    return useMutation({
        mutationFn: (req) => {
            return bodhveda.recipients.preferences.set(recipientID, req);
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: [QUERY_KEYS.usePreferences],
            });
            queryClient.invalidateQueries({
                queryKey: [QUERY_KEYS.useCheckPreference, args[0].target],
            });
            onSuccess === null || onSuccess === void 0 ? void 0 : onSuccess(...args);
        },
        ...rest,
    });
}
export function useCheckPreference(target, options = {}) {
    const bodhveda = useBodhveda();
    const recipientID = useRecipientID();
    return useQuery({
        queryKey: [QUERY_KEYS.useCheckPreference, target],
        queryFn: () => bodhveda.recipients.preferences.check(recipientID, { target }),
        ...options,
    });
}
//# sourceMappingURL=hooks.js.map