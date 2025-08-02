import {
    QueryCache,
    QueryClient,
    QueryClientProvider,
} from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";

import { routeTree } from "@/routeTree.gen";
import { useAuth, AuthProvider } from "@/features/auth/auth_context";
import { apiErrorHandler } from "@/lib/api";
import { LoadingScreen } from "./components/loading_screen";
import { SidebarProvider } from "./components/sidebar/sidebar";

// Create a new router instance
const router = createRouter({
    routeTree,
    context: {
        auth: undefined!, // This will be set after we wrap the app in an AuthProvider
    },
});

// Register the router instance for type safety
declare module "@tanstack/react-router" {
    interface Register {
        router: typeof router;
    }
}

function InnerApp() {
    const auth = useAuth();

    if (auth.isLoading) {
        return (
            <div className="h-screen w-screen">
                <LoadingScreen />
            </div>
        );
    }

    return <RouterProvider router={router} context={{ auth }} />;
}

const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            retry: false,
            refetchOnWindowFocus: false,
        },
    },

    queryCache: new QueryCache({
        onError: (error) => {
            apiErrorHandler(error);
        },
    }),
});

export function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <AuthProvider>
                <SidebarProvider>
                    <InnerApp />
                </SidebarProvider>
            </AuthProvider>
        </QueryClientProvider>
    );
}
