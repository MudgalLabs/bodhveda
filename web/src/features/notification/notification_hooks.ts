import {
    AnyUseMutationOptions,
    useMutation,
    useQueryClient,
} from "@tanstack/react-query";

import { API_ROUTES, APIRes, client } from "@/lib/api";
import {
    SendNotificationPayload,
    SendNotificationResponse,
} from "@/features/home/notification_types";
import { getRecipientsKey } from "../recipient/recipient_hooks";

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
