import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";
import { client, API_ROUTES, APIRes } from "@/lib/api";
import {
    ProjectPreference,
    CreateProjectPreferencePayload,
} from "./preference_type";

export function useGetProjectPreferences(projectID: string) {
    return useQuery({
        queryKey: ["useGetProjectPreferences", projectID],
        queryFn: () =>
            client.get(API_ROUTES.project.preferences.list(projectID)),
        select: (res) => res.data as APIRes<ProjectPreference[]>,
    });
}

export function useCreateProjectPreference(
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<ProjectPreference>,
        unknown,
        { projectID: string; payload: CreateProjectPreferencePayload }
    >({
        mutationFn: ({ projectID, payload }) => {
            return client.post(
                API_ROUTES.project.preferences.create(projectID),
                payload
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: ["useGetProjectPreferences"],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
