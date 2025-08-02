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
