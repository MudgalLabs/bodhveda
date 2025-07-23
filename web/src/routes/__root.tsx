import { Outlet, createRootRouteWithContext } from "@tanstack/react-router";
import { TanStackRouterDevtools } from "@tanstack/react-router-devtools";

import type { AuthContextType } from "@/features/auth/auth_context";

interface MyRouterContext {
    auth: AuthContextType;
}

export const Route = createRootRouteWithContext<MyRouterContext>()({
    component: () => (
        <>
            <Outlet />
            <TanStackRouterDevtools
                position="bottom-right"
                initialIsOpen={false}
            />
        </>
    ),
});
