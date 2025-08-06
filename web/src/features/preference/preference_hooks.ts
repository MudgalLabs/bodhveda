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
    PreferenceKind,
    RecipientPreference,
} from "./preference_type";

export function useGetPreferences(projectID: string, kind: PreferenceKind) {
    return useQuery({
        queryKey: ["useGetPreferences", projectID, kind],
        queryFn: () =>
            client.get(API_ROUTES.project.preferences.list(projectID), {
                params: { kind },
            }),
        select: (res) => {
            if (kind === "project") {
                return res.data as APIRes<ProjectPreference[]>;
            } else {
                return res.data as APIRes<RecipientPreference[]>;
            }
        },
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
