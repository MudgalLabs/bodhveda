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
    UpsertRecipientPreferencePayload,
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

// useGetRecipientPreferences reads ONE recipient's RESOLVED preferences: every
// (target, active medium) answered by the same cascade the send path uses, so
// each cell states what a send would actually do.
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

/**
 * Toggles ONE (target, medium) for one recipient through the existing console
 * PUT. One cell = one call; the endpoint upserts.
 *
 * On success it invalidates that recipient's resolved read rather than patching
 * the cache: a write can move cells it did not target (a topic='any' rule
 * decides every exact-topic cell in its channel/event that has no row of its
 * own), and only the server resolves the cascade. Re-reading is the honest move.
 */
export function useUpsertRecipientPreference(
    projectID: string,
    recipientID: string,
    options: AnyUseMutationOptions = {}
) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<
        APIRes<RecipientPreference>,
        unknown,
        UpsertRecipientPreferencePayload
    >({
        mutationFn: (payload) =>
            client.put(
                API_ROUTES.project.recipients.preferences(
                    projectID,
                    recipientID
                ),
                payload
            ),
        // Awaited so the refetched read is in cache before callers run. A caller
        // dropping its optimistic state any earlier would flash the stale value.
        onSuccess: async (...args) => {
            await queryClient.invalidateQueries({
                queryKey: getRecipientPreferencesKey(projectID, recipientID),
            });
            onSuccess?.(...args);
        },
        ...rest,
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
