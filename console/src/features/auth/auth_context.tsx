import {
    createContext,
    FC,
    PropsWithChildren,
    useContext,
    useEffect,
    useMemo,
    useState,
} from "react";

import { useGetMe, useLogout } from "@/features/auth/auth_hooks";
import { User } from "@/features/auth/auth_types";
import { useQueryClient } from "@tanstack/react-query";

export interface AuthContextType {
    isLoading: boolean;
    isAuthenticated: boolean;
    user: User | undefined;
    logout: () => void;
}

const AuthContext = createContext<AuthContextType>({
    isAuthenticated: false,
    isLoading: true,
    user: undefined,
    logout: () => {},
});

export const AuthProvider: FC<PropsWithChildren> = ({ children }) => {
    const [user, setUser] = useState<User | undefined>(undefined);
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [isLoading, setIsLoading] = useState(true);

    const { data, isSuccess, isLoading: getMeIsLoading } = useGetMe();
    const queryClient = useQueryClient();

    const { mutate: logout } = useLogout({
        onSuccess: async () => {
            await queryClient.invalidateQueries();
            setIsAuthenticated(false);
            setUser(undefined);
        },
    });

    useEffect(() => {
        if (getMeIsLoading) {
            setIsLoading(true);
            return;
        }

        if (isSuccess) {
            setIsAuthenticated(true);
            setUser(data?.data);
            setIsLoading(false);
        } else {
            setIsAuthenticated(false);
            setUser(undefined);
            setIsLoading(false);
        }
    }, [data?.data, isSuccess, getMeIsLoading]);

    const value = useMemo(
        () => ({
            isLoading,
            isAuthenticated,
            user,
            logout,
        }),
        [isLoading, isAuthenticated, user, logout]
    );

    return (
        <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
    );
};

export function useAuth(): AuthContextType {
    const context = useContext(AuthContext);

    if (!context) {
        throw new Error(
            "useAuthentication: did you forget to use AuthProvider?"
        );
    }

    return context;
}
