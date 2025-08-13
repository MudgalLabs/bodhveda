import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";

import { client, API_ROUTES, APIRes } from "@/lib/api";
import { APIKey, CreateAPIKeyPayload } from "@/features/api_key/api_key_types";

export function useGetAPIKeys(projectID: string) {
    return useQuery({
        queryKey: ["useGetAPIKeys"],
        queryFn: () => client.get(API_ROUTES.project.api_keys.list(projectID)),
        select: (res) => res.data as APIRes<APIKey[]>,
    });
}

export function useCreateAPIKey(options: AnyUseMutationOptions = {}) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<string>,
        unknown,
        { projectID: string; payload: CreateAPIKeyPayload }
    >({
        mutationFn: ({ projectID, payload }) => {
            return client.post(
                API_ROUTES.project.api_keys.create(projectID),
                payload
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({ queryKey: ["useGetAPIKeys"] });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useDeleteAPIKey(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<APIRes<string>, unknown, { apiKeyID: number }>({
        mutationFn: ({ apiKeyID }) => {
            return client.delete(
                API_ROUTES.project.api_keys.delete(projectID, apiKeyID)
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({ queryKey: ["useGetAPIKeys"] });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
