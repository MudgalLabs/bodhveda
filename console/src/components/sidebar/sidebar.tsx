import { useEffect, useState } from "react";
import { useLocation, Link } from "@tanstack/react-router";

import { cn } from "@/lib/utils";
import { useAuth } from "@/features/auth/auth_context";
import {
    buttonVariants,
    IconArrowLeft,
    IconLogout,
    IconKey,
    IconUsers,
    IconSlidersHorizontal,
    IconDashboard,
    IconBell,
    IconSend,
    useSidebar,
    SidebarItem,
    useIsMobile,
    IconCreditCard,
} from "netra";
import {
    useGetProjectIDFromParams,
    useGetProjects,
} from "@/features/project/project_hooks";
import { Branding } from "@/components/branding";
import { DEFAULT_RANGE_PRESET } from "@/features/dashboard/analytics_range";
import { DEFAULT_NOTIFICATION_KIND } from "@/features/notification/notification_types";
import { DEFAULT_PREFERENCE_KIND } from "@/features/preference/preference_type";

export const Sidebar = () => {
    const { pathname } = useLocation();
    const { isOpen, setIsOpen } = useSidebar();
    const isMobile = useIsMobile();

    const { logout } = useAuth();
    const id = useGetProjectIDFromParams();

    // The current project's name, for the "back to projects" item — it doubles as
    // a breadcrumb showing which project you're in. Falls back to "Projects" until
    // the (cached) projects list resolves.
    const { data: projects } = useGetProjects();
    const projectName =
        projects?.data.find((p) => String(p.id) === id)?.name ?? "Projects";

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

                        <div className="h-4" />

                        {/* Only the arrow is the link back to projects; the
                            project name beside it is a non-clickable breadcrumb,
                            pushed to the right edge of the sidebar. The Link wears
                            netra's `link` button styling so it reads as a link. */}
                        <div className="flex items-center justify-between gap-x-2">
                            <Link
                                to="/projects"
                                aria-label="Back to projects"
                                className={buttonVariants({
                                    variant: "link",
                                    size: "icon",
                                })}
                            >
                                <IconArrowLeft size={18} />
                            </Link>
                            {isOpen && !isMobile && (
                                <span
                                    className="text-text-muted min-w-0 truncate text-sm font-medium"
                                    title={projectName}
                                >
                                    {projectName}
                                </span>
                            )}
                        </div>
                    </div>

                    <Link
                        to="/projects/$id/dashboard"
                        params={{ id }}
                        search={{ preset: DEFAULT_RANGE_PRESET }}
                        className="link-unstyled "
                    >
                        <SidebarItem
                            label="Dashboard"
                            icon={<IconDashboard size={18} />}
                            open={isOpen}
                            isActive={
                                activeRoute === `/projects/${id}/dashboard`
                            }
                        />
                    </Link>

                    <Link
                        to="/projects/$id/notifications"
                        params={{ id }}
                        search={{ kind: DEFAULT_NOTIFICATION_KIND }}
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
                        search={{ kind: DEFAULT_PREFERENCE_KIND }}
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

                    <Link
                        to="/projects/$id/settings"
                        params={{ id }}
                        className="link-unstyled "
                    >
                        <SidebarItem
                            label="Email"
                            icon={<IconSend size={18} />}
                            open={isOpen}
                            isActive={
                                activeRoute === `/projects/${id}/settings`
                            }
                        />
                    </Link>
                </div>
            </div>

            <div className="mb-4 space-y-2">
                <Link
                    to="/projects/$id/billing"
                    params={{ id }}
                    className="link-unstyled "
                >
                    <SidebarItem
                        label="Billing"
                        icon={<IconCreditCard size={18} />}
                        open={isOpen}
                        isActive={activeRoute === `/projects/${id}/billing`}
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
