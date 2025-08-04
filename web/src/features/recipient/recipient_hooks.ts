import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { client, API_ROUTES, APIRes } from "@/lib/api";
import { Recipient, CreateRecipientPayload } from "./recipient_types";

export function useGetRecipients(projectID: string) {
    return useQuery({
        queryKey: ["useGetRecipients", projectID],
        queryFn: () =>
            client.get(API_ROUTES.project.recipients.list(projectID)),
        select: (res) => res.data as APIRes<Recipient[]>,
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
            queryClient.invalidateQueries({ queryKey: ["useGetRecipients"] });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
