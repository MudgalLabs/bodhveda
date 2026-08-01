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
    ProjectListItem,
    UpdateProjectPayload,
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
        select: (res) => res.data as APIRes<ProjectListItem[]>,
    });
}

export function useGetProject(id: string | number) {
    return useQuery({
        queryKey: ["useGetProject", String(id)],
        queryFn: () => client.get(API_ROUTES.project.get(id)),
        select: (res) => res.data as APIRes<Project>,
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

export function useUpdateProject(options: AnyUseMutationOptions = {}) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<APIRes<Project>, unknown, UpdateProjectPayload>({
        // `strict_targets` is forwarded only when the caller set it. Spreading a
        // key that is `undefined` would serialise to nothing anyway, but being
        // explicit keeps the "omitted means unchanged" contract visible here,
        // where the rename dialog's payload is built.
        mutationFn: ({ id, name, strict_targets }) => {
            return client.patch(API_ROUTES.project.update(id), {
                name,
                ...(strict_targets === undefined ? {} : { strict_targets }),
            });
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({ queryKey: ["useGetProjects"] });
            queryClient.invalidateQueries({ queryKey: ["useGetProject"] });
            onSuccess?.(...args);
        },
        ...rest,
    });
}

export function useDeleteProject(options: AnyUseMutationOptions = {}) {
    const { onSuccess, ...rest } = options;
    const queryClient = useQueryClient();

    return useMutation<APIRes<void>, unknown, number>({
        mutationFn: (id) => {
            return client.delete(API_ROUTES.project.delete(id));
        },
        onSuccess: (...args) => {
            queryClient.invalidateQueries({ queryKey: ["useGetProjects"] });
            onSuccess?.(...args);
        },
        ...rest,
    });
}
