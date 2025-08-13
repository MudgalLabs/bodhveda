import {
    FC,
    useEffect,
    useState,
    createContext,
    PropsWithChildren,
    useContext,
} from "react";
import { useLocation, Link } from "@tanstack/react-router";
import { IconType } from "react-icons";

import { cn } from "@/lib/utils";
import { useAuth } from "@/features/auth/auth_context";
import {
    IconArrowLeft,
    IconLogout,
    IconKey,
    IconUsers,
    IconSlidersHorizontal,
    IconHouse,
    Tooltip,
    IconBell,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { Branding } from "@/components/branding";

export const Sidebar = () => {
    const { pathname } = useLocation();
    const { isOpen, setIsOpen } = useSidebar();
    const { logout } = useAuth();
    const id = useGetProjectIDFromParams();

    const [activeRoute, setActiveRoute] = useState("");
    useEffect(() => {
        setActiveRoute(pathname);
    }, [pathname, setIsOpen]);

    return (
        <div
            className={cn(
                "relative flex h-full flex-col justify-between px-3",
                {
                    "w-[240px]!": isOpen,
                }
            )}
        >
            <div>
                <div className="mt-6 flex flex-col gap-y-2 pb-2">
                    <div className="flex-x mb-8 ml-2 justify-between">
                        <Link
                            to="/projects/$id/home"
                            params={{ id }}
                            className="link-unstyled cursor-pointer!"
                        >
                            <Branding
                                size="small"
                                hideBetaTag={!isOpen}
                                hideText
                            />
                        </Link>
                    </div>

                    <Link
                        to="/projects/$id/home"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarNavItem
                            label="Home"
                            Icon={IconHouse}
                            open={isOpen}
                            isActive={activeRoute === `/projects/${id}/home`}
                        />
                    </Link>

                    <Link
                        to="/projects/$id/notifications"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarNavItem
                            label="Notifications"
                            Icon={IconBell}
                            open={isOpen}
                            isActive={
                                activeRoute === `/projects/${id}/notifications`
                            }
                        />
                    </Link>

                    <Link
                        to="/projects/$id/recipients"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarNavItem
                            label="Recipients"
                            Icon={IconUsers}
                            open={isOpen}
                            isActive={
                                activeRoute === `/projects/${id}/recipients`
                            }
                        />
                    </Link>

                    <Link
                        to="/projects/$id/preferences"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarNavItem
                            label="Preferences"
                            Icon={IconSlidersHorizontal}
                            open={isOpen}
                            isActive={
                                activeRoute === `/projects/${id}/preferences`
                            }
                        />
                    </Link>

                    <Link
                        to="/projects/$id/api-keys"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarNavItem
                            label="API Keys"
                            Icon={IconKey}
                            open={isOpen}
                            isActive={
                                activeRoute === `/projects/${id}/api-keys`
                            }
                        />
                    </Link>
                </div>
            </div>

            <div className="mb-4 space-y-2">
                <Link to="/projects" className="link-unstyled ">
                    <SidebarNavItem
                        label="Back to Projects"
                        Icon={IconArrowLeft}
                        open={isOpen}
                        isActive={activeRoute === `/projects/${id}/projects`}
                    />
                </Link>

                <SidebarNavItem
                    label="Logout"
                    Icon={IconLogout}
                    open={isOpen}
                    onClick={() => logout()}
                />
            </div>
        </div>
    );
};

interface SidebarNavItemProps {
    label: string;
    Icon: IconType;
    open?: boolean;
    isActive?: boolean;
    onClick?: () => void;
}

const SidebarNavItem: FC<SidebarNavItemProps> = (props) => {
    const { label, Icon, open, isActive, onClick } = props;

    const content = (
        <div
            className={cn(
                "peer text-text-muted [&_svg]:text-text-muted hover:[&_svg]:text-text-primary w-full rounded-sm bg-transparent p-2 transition-colors",
                {
                    "bg-secondary-hover text-text-primary": isActive,
                    "hover:bg-secondary-hover hover:text-text-primary":
                        !isActive,
                    "flex items-center gap-2 text-base": open,
                    "mx-auto flex h-9 w-9 items-center justify-center": !open,
                }
            )}
            onClick={onClick}
        >
            <Icon size={20} />
            {open && <p className="text-sm">{label}</p>}
        </div>
    );

    return (
        <Tooltip
            content={label}
            delayDuration={0}
            contentProps={{ side: "right" }}
            disabled={open}
        >
            {content}
        </Tooltip>
    );
};

interface SidebarContextType {
    isOpen: boolean;
    setIsOpen: (isOpen: boolean) => void;
    toggleSidebar: () => void;
}

const SidebarContext = createContext<SidebarContextType>({
    isOpen: false,
    setIsOpen: () => {},
    toggleSidebar: () => {},
});

export const SidebarProvider: FC<PropsWithChildren> = ({ children }) => {
    const [isOpen, setIsOpen] = useState(true);

    function toggleSidebar() {
        setIsOpen((prev) => !prev);
    }

    return (
        <SidebarContext.Provider
            value={{
                isOpen,
                setIsOpen,
                toggleSidebar,
            }}
        >
            {children}
        </SidebarContext.Provider>
    );
};

export function useSidebar(): SidebarContextType {
    const context = useContext(SidebarContext);

    if (!context) {
        throw new Error("useSidebar: did you forget to use SidebarProvider?");
    }

    return context;
}
