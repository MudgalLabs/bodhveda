import { useEffect, useState } from "react";
import { useLocation, Link } from "@tanstack/react-router";

import { cn } from "@/lib/utils";
import { useAuth } from "@/features/auth/auth_context";
import {
    IconArrowLeft,
    IconLogout,
    IconKey,
    IconUsers,
    IconSlidersHorizontal,
    IconHouse,
    IconBell,
    useSidebar,
    SidebarItem,
    useIsMobile,
} from "netra";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { Branding } from "@/components/branding";

export const Sidebar = () => {
    const { pathname } = useLocation();
    const { isOpen, setIsOpen } = useSidebar();
    const isMobile = useIsMobile();

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
                    "w-[220px]!": isOpen && !isMobile,
                    hidden: !isOpen && isMobile,
                }
            )}
        >
            <div>
                <div className="mt-6 flex flex-col gap-y-2 pb-2">
                    <div className="mb-8">
                        <Branding
                            size="small"
                            hideText={!isOpen || isMobile}
                            hideBetaTag={!isOpen || isMobile}
                        />
                    </div>

                    <Link
                        to="/projects/$id/home"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarItem
                            label="Home"
                            icon={<IconHouse size={18} />}
                            open={isOpen}
                            isActive={activeRoute === `/projects/${id}/home`}
                        />
                    </Link>

                    <Link
                        to="/projects/$id/notifications"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarItem
                            label="Notifications"
                            icon={<IconBell size={18} />}
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
                        <SidebarItem
                            label="Recipients"
                            icon={<IconUsers size={18} />}
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
                        <SidebarItem
                            label="Preferences"
                            icon={<IconSlidersHorizontal size={18} />}
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
                        <SidebarItem
                            label="API Keys"
                            icon={<IconKey size={18} />}
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
                    <SidebarItem
                        label="Back to Projects"
                        icon={<IconArrowLeft size={18} />}
                        open={isOpen}
                        isActive={activeRoute === `/projects/${id}/projects`}
                    />
                </Link>

                <SidebarItem
                    label="Logout"
                    icon={<IconLogout size={18} />}
                    open={isOpen}
                    onClick={() => logout()}
                    isActive={false}
                />
            </div>
        </div>
    );
};

// interface SidebarNavItemProps {
//     label: string;
//     Icon: IconType;
//     open?: boolean;
//     isActive?: boolean;
//     onClick?: () => void;
// }

// const SidebarNavItem: FC<SidebarNavItemProps> = (props) => {
//     const { label, Icon, open, isActive, onClick } = props;

//     const content = (
//         <div
//             className={cn(
//                 "peer text-text-muted [&_svg]:text-text-muted hover:[&_svg]:text-text-primary w-full rounded-sm bg-transparent p-2 transition-colors",
//                 {
//                     "bg-secondary-hover text-text-primary": isActive,
//                     "hover:bg-secondary-hover hover:text-text-primary":
//                         !isActive,
//                     "flex items-center gap-2 text-base": open,
//                     "mx-auto flex h-9 w-9 items-center justify-center": !open,
//                 }
//             )}
//             onClick={onClick}
//         >
//             <Icon size={20} />
//             {open && <p className="text-sm">{label}</p>}
//         </div>
//     );

//     return (
//         <Tooltip
//             content={label}
//             delayDuration={0}
//             contentProps={{ side: "right" }}
//             disabled={open}
//         >
//             {content}
//         </Tooltip>
//     );
// };
