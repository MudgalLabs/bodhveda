import axios, { AxiosError, AxiosResponse } from "axios";

import { isProd } from "@/lib/utils";
import { toast } from "@/components/toast";

export const API_ROUTES = {
    auth: {
        signout: "/v1/platform/auth/sign-out",
    },
    user: {
        me: "/v1/platform/users/me",
    },
};

let API_URL = import.meta.env.API_URL;

if (!API_URL) {
    if (isProd()) {
        throw new Error("API's URL is not set");
    } else {
        API_URL = "http://localhost:1338";
    }
}

export const client = createAPIClient(API_URL);

/** This is the API's response structure. */
export interface APIRes<T = unknown> {
    status: "success" | "error";
    message: string;
    errors: ApiResError[];
    data: T;
}

/** This is API's error object strucutre. */
interface ApiResError {
    message: string;
    description: string;
}

function createAPIClient(baseURL: string) {
    const client = axios.create({
        baseURL,
        withCredentials: true,
    });

    client.interceptors.response.use(
        (res: AxiosResponse) => {
            return res;
        },
        async (err: AxiosError) => {
            const status = err.response ? err.response.status : null;

            if (status === 401 || status === 403) {
                if (window.location.pathname !== "/login") {
                    // Redirect to sign-in page if the user is not authenticated.
                    window.history.pushState({}, "", "/login");
                }
            }

            return Promise.reject(err);
        }
    );

    return client;
}

export function apiErrorHandler(err: any) {
    const _err = err as AxiosError<APIRes>;

    // Don't show toast for cancelled requests
    if (_err.code === "ERR_CANCELED" || _err.name === "CanceledError") {
        return;
    }

    const DEFAULT_MESSAGE = "Something went wrong. Please try again.";
    let message = DEFAULT_MESSAGE;

    if (_err.name === "AxiosError") {
        if (_err.status) {
            // We are not logged in. We don't want to bombard user with this error toast.
            if (_err.status === 401) return;

            // We don't want to leak(to UI) anything if the server messed up.
            if (_err.status >= 300 && _err.status < 500) {
                if (_err.response?.data.message) {
                    message = _err.response.data.message;
                }

                if (_err.status === 429) {
                    toast.error("You’re going too fast!", {
                        description: "Give it a sec and try again.",
                    });
                    return;
                }
            }
        }
    }

    toast.error(message);
}
