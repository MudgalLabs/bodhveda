import {
    Card,
    CardContent,
    CardTitle,
    ErrorMessage,
    formatNumber,
    IconBell,
    IconMegaphone,
    IconTarget,
    PageHeading,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useNotificationsOverview } from "@/features/notification/notification_hooks";
import { ReactNode, useMemo } from "react";

export function NotificationList() {
    const projectID = useGetProjectIDFromParams();
    const { data, isLoading, isError } = useNotificationsOverview(projectID);

    const content = useMemo(() => {
        if (isError) {
            return <ErrorMessage errorMsg="Error loading notifications" />;
        }

        if (!data) return null;

        return (
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 p-4">
                <OverviewCard
                    title="Total notifications delivered"
                    icon={<IconBell size={24} />}
                    count={data.total_notifications}
                />
                <OverviewCard
                    title="Direct notifications sent"
                    icon={<IconTarget size={24} />}
                    count={data.total_direct_sent}
                />
                <OverviewCard
                    title="Broadcast notifications sent"
                    icon={<IconMegaphone size={24} />}
                    count={data.total_broadcast_sent}
                />
            </div>
        );
    }, [data, isError]);

    return (
        <div>
            <PageHeading heading="Notifications" loading={isLoading} />

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
