import {
    Card,
    CardContent,
    CardTitle,
    ErrorMessage,
    formatNumber,
    PageHeading,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useNotificationsOverview } from "@/features/notification/notification_hooks";
import { useMemo } from "react";

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
                    title="Total Notifications"
                    emoji="📬"
                    count={data.total_notifications}
                />
                <OverviewCard
                    title="Direct Notifications Sent"
                    emoji="🎯"
                    count={data.total_direct_sent}
                />
                <OverviewCard
                    title="Broadcast Notifications Sent"
                    emoji="📢"
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
    emoji: string;
    count: number;
}

function OverviewCard(props: OverviewCardProps) {
    return (
        <Card>
            <CardTitle className="flex-x justify-between">
                <span>{props.title}</span>
                <span className="sub-heading">{props.emoji}</span>
            </CardTitle>
            <CardContent>
                <div className="big-heading">{formatNumber(props.count)}</div>
            </CardContent>
        </Card>
    );
}
