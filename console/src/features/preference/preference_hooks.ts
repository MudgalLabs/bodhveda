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
    RecipientPreferenceTargetStatesResult,
} from "@/features/preference/preference_type";

export function useGetPreferences(projectID: string, kind: PreferenceKind) {
    return useQuery({
        queryKey: getProjectPreferencesKey(projectID, kind),
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

// useGetRecipientPreferences reads ONE recipient's resolved preferences: the
// project catalog overlaid with that recipient's own overrides. Read-only in
// 9.2 — Phase 9.3 turns this into the editable per-(target, medium) grid.
export function useGetRecipientPreferences(
    projectID: string,
    recipientID: string
) {
    return useQuery({
        queryKey: getRecipientPreferencesKey(projectID, recipientID),
        queryFn: () =>
            client.get(
                API_ROUTES.project.recipients.preferences(projectID, recipientID)
            ),
        select: (res) =>
            res.data as APIRes<RecipientPreferenceTargetStatesResult>,
        enabled: !!projectID && !!recipientID,
    });
}

export function getRecipientPreferencesKey(
    projectID?: string,
    recipientID?: string
) {
    if (projectID && recipientID) {
        return ["useGetRecipientPreferences", projectID, recipientID];
    }
    return ["useGetRecipientPreferences"];
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
                predicate: (query) =>
                    Array.isArray(query.queryKey) &&
                    query.queryKey[0] === getProjectPreferencesKey()[0],
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useDeleteProjectPreference(
    projectID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<ProjectPreference>,
        unknown,
        { preferenceID: number }
    >({
        mutationFn: ({ preferenceID }) => {
            return client.delete(
                API_ROUTES.project.preferences.delete(projectID, preferenceID)
            );
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({
                queryKey: getProjectPreferencesKey(projectID),
            });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

function getProjectPreferencesKey(projectID?: string, kind?: PreferenceKind) {
    if (projectID && kind) {
        return ["useGetProjectPreferences", projectID, kind];
    }
    return ["useGetProjectPreferences"];
}
