import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { client, API_ROUTES, APIRes } from "@/lib/api";
import {
    CreateRecipientContactPayload,
    ListRecipientContactsResult,
    RecipientContact,
    UpdateRecipientContactPayload,
} from "@/features/recipient/contact_types";

export function getRecipientContactsKey(
    projectID?: string,
    recipientID?: string
) {
    if (projectID && recipientID) {
        return ["useGetRecipientContacts", projectID, recipientID];
    }
    return ["useGetRecipientContacts"];
}

export function useGetRecipientContacts(
    projectID: string,
    recipientID: string,
    enabled = true
) {
    return useQuery({
        queryKey: getRecipientContactsKey(projectID, recipientID),
        queryFn: () =>
            client.get(
                API_ROUTES.project.recipients.contacts.list(
                    projectID,
                    recipientID
                )
            ),
        select: (res) => res.data as APIRes<ListRecipientContactsResult>,
        enabled: enabled && !!projectID && !!recipientID,
    });
}

export function useCreateRecipientContact(
    projectID: string,
    recipientID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<RecipientContact>,
        unknown,
        { payload: CreateRecipientContactPayload }
    >({
        mutationFn: ({ payload }) =>
            client.post(
                API_ROUTES.project.recipients.contacts.create(
                    projectID,
                    recipientID
                ),
                payload
            ),
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getRecipientContactsKey(projectID, recipientID),
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useUpdateRecipientContact(
    projectID: string,
    recipientID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<RecipientContact>,
        unknown,
        { contactID: number; payload: UpdateRecipientContactPayload }
    >({
        mutationFn: ({ contactID, payload }) =>
            client.patch(
                API_ROUTES.project.recipients.contacts.update(
                    projectID,
                    recipientID,
                    contactID
                ),
                payload
            ),
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getRecipientContactsKey(projectID, recipientID),
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useDeleteRecipientContact(
    projectID: string,
    recipientID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<APIRes<null>, unknown, { contactID: number }>({
        mutationFn: ({ contactID }) =>
            client.delete(
                API_ROUTES.project.recipients.contacts.delete(
                    projectID,
                    recipientID,
                    contactID
                )
            ),
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getRecipientContactsKey(projectID, recipientID),
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
