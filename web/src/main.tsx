import { StrictMode } from "react";
import ReactDOM from "react-dom/client";
import {
    QueryCache,
    QueryClient,
    QueryClientProvider,
} from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";

import { routeTree } from "@/routeTree.gen";
import { useAuth, AuthProvider } from "@/features/auth/auth_context";
import { apiErrorHandler } from "@/lib/api";

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
        return <div>Loading...</div>;
    }

    console.log("Auth context:", auth);

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

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <AuthProvider>
                <InnerApp />
            </AuthProvider>
        </QueryClientProvider>
    );
}

// Render the app
const rootElement = document.getElementById("root")!;

if (!rootElement.innerHTML) {
    const root = ReactDOM.createRoot(rootElement);
    root.render(
        <StrictMode>
            <App />
        </StrictMode>
    );
}
