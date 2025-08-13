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
    EditRecipientPayload,
    ListRecipientsResult,
} from "@/features/recipient/recipient_types";

export function useGetRecipients(
    projectID: string,
    page: number,
    limit: number
) {
    return useQuery({
        queryKey: getRecipientsKey(projectID, page, limit),
        queryFn: () =>
            client.get(API_ROUTES.project.recipients.list(projectID), {
                params: { page, limit },
            }),
        select: (res) => res.data as APIRes<ListRecipientsResult>,
    });
}

export function useCreateRecipient(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<Recipient>,
        unknown,
        { payload: CreateRecipientPayload }
    >({
        mutationFn: ({ payload }) => {
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

export function useDeleteRecipient(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<APIRes<Recipient>, unknown, { recipientID: string }>({
        mutationFn: ({ recipientID }) => {
            return client.delete(
                API_ROUTES.project.recipients.delete(projectID, recipientID)
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getRecipientsKey(projectID),
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useEditRecipient(
    projectID: string,
    recipientID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<Recipient>,
        unknown,
        { payload: EditRecipientPayload }
    >({
        mutationFn: ({ payload }) => {
            return client.patch(
                API_ROUTES.project.recipients.edit(projectID, recipientID),
                payload
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getRecipientsKey(projectID),
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function getRecipientsKey(
    projectID?: string,
    page?: number,
    limit?: number
) {
    if (projectID) {
        return ["useGetRecipients", projectID, page, limit];
    } else {
        return ["useGetRecipients"];
    }
}
