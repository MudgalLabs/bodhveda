import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { client, API_ROUTES, APIRes } from "@/lib/api";
import {
    Recipient,
    CreateRecipientPayload,
    RecipientListItem,
} from "./recipient_types";

export function useGetRecipients(projectID: string) {
    return useQuery({
        queryKey: getRecipientsKey(projectID),
        queryFn: () =>
            client.get(API_ROUTES.project.recipients.list(projectID)),
        select: (res) => res.data as APIRes<RecipientListItem[]>,
    });
}

export function useCreateRecipient(options: AnyUseMutationOptions = {}) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<Recipient>,
        unknown,
        { projectID: string; payload: CreateRecipientPayload }
    >({
        mutationFn: ({ projectID, payload }) => {
            return client.post(
                API_ROUTES.project.recipients.create(projectID),
                payload
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                predicate: (query) =>
                    Array.isArray(query.queryKey) &&
                    query.queryKey[0] === getRecipientsKey()[0],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

function getRecipientsKey(projectID?: string) {
    if (projectID) {
        return ["useGetRecipients", projectID];
    } else {
        return ["useGetRecipients"];
    }
}
