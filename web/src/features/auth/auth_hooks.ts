import {
    AnyUseMutationOptions,
    useMutation,
    useQuery,
} from "@tanstack/react-query";

import { client, API_ROUTES, APIRes } from "@/lib/api";
import { User } from "@/features/auth/auth_types";

export function useGetMe() {
    return useQuery({
        queryKey: ["useGetMe"],
        queryFn: () => client.get(API_ROUTES.user.me),
        select: (res) => res.data as APIRes<User>,
    });
}

export function useLogout(options: AnyUseMutationOptions = {}) {
    return useMutation<APIRes, unknown, void, unknown>({
        mutationFn: () => {
            return client.post(API_ROUTES.auth.signout);
        },
        ...options,
    });
}
