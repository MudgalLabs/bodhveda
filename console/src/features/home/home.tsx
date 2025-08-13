import {
    Card,
    CardContent,
    CardTitle,
    ErrorMessage,
    formatNumber,
    IconBell,
    IconMegaphone,
    IconTarget,
    IconUsers,
    PageHeading,
} from "netra";

import {
    useGetProjectIDFromParams,
    useGetProjects,
} from "@/features/project/project_hooks";
import { ReactNode, useMemo } from "react";
import { useSidebar } from "@/components/sidebar/sidebar";

export function Home() {
    const { isOpen, toggleSidebar } = useSidebar();
    const projectID = useGetProjectIDFromParams();
    const { data: projects, isLoading, isError } = useGetProjects();

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading notifications" />;
        }

        if (!projects) return null;

        const data = projects.data.find((p) => String(p.id) === projectID);

        if (!data) return <ErrorMessage errorMsg="Project not found" />;

        return (
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4 p-4">
                <OverviewCard
                    title="Recipients"
                    icon={<IconUsers size={24} />}
                    count={data.total_recipients}
                />
                <OverviewCard
                    title="Notifications"
                    icon={<IconBell size={24} />}
                    count={data.total_notifications}
                />
                <OverviewCard
                    title="Direct"
                    icon={<IconTarget size={24} />}
                    count={data.total_direct_sent}
                />
                <OverviewCard
                    title="Broadcast"
                    icon={<IconMegaphone size={24} />}
                    count={data.total_broadcast_sent}
                />
            </div>
        );
    }, [isError, projectID, projects]);

    return (
        <div>
            <PageHeading
                heading="Home"
                loading={isLoading}
                isOpen={isOpen}
                toggleSidebar={toggleSidebar}
            />

            {content}
        </div>
    );
}

interface OverviewCardProps {
    title: string;
    icon: ReactNode;
    count: number;
}

function OverviewCard(props: OverviewCardProps) {
    return (
        <Card>
            <CardTitle className="flex-x justify-between">
                <span className="font-semibold">{props.title}</span>
                <span>{props.icon}</span>
            </CardTitle>
            <CardContent>
                <div className="big-heading">{formatNumber(props.count)}</div>
            </CardContent>
        </Card>
    );
}
