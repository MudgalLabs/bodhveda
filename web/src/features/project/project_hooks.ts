import { useParams } from "@tanstack/react-router";
import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
    useQueryClient,
} from "@tanstack/react-query";

import { client, API_ROUTES, APIRes } from "@/lib/api";
import {
    Project,
    CreateProjectPayload,
} from "@/features/project/project_types";

// This hook retrieves the project ID from the URL parameters.
// NOTE: This should be used on pages that are under the `/projects/$id` route.
export function useGetProjectIDFromParams(): string {
    const { id } = useParams({ from: "/projects/$id" });
    return id;
}

export function useGetProjects() {
    return useQuery({
        queryKey: ["useGetProjects"],
        queryFn: () => client.get(API_ROUTES.project.list),
        select: (res) => res.data as APIRes<Project[]>,
    });
}

export function useCreateProject(options: AnyUseMutationOptions = {}) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<APIRes<Project>, unknown, CreateProjectPayload>({
        mutationFn: (payload) => {
            return client.post(API_ROUTES.project.create, payload);
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({ queryKey: ["useGetProjects"] });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
