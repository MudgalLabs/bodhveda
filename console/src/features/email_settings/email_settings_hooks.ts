import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";

import { client, API_ROUTES, APIRes } from "@/lib/api";
import {
    ProjectEmailSettings,
    UpsertProjectEmailSettingsPayload,
} from "@/features/email_settings/email_settings_types";

export function useGetEmailSettings(projectID: string) {
    return useQuery({
        queryKey: ["useGetEmailSettings", projectID],
        queryFn: () =>
            client.get(API_ROUTES.project.email_settings.get(projectID)),
        // `data` is null when the project has no email settings configured yet.
        select: (res) => res.data as APIRes<ProjectEmailSettings | null>,
    });
}

export function useUpsertEmailSettings(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<ProjectEmailSettings>,
        unknown,
        UpsertProjectEmailSettingsPayload
    >({
        mutationFn: (payload) =>
            client.put(
                API_ROUTES.project.email_settings.upsert(projectID),
                payload
            ),
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: ["useGetEmailSettings", projectID],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
