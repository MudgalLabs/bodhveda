import {
    Card,
    CardContent,
    CardTitle,
    ErrorMessage,
    formatCurrency,
    LoadingScreen,
    PageHeading,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useNotificationsOverview } from "@/features/notification/notification_hooks";

export function NotificationList() {
    const projectID = useGetProjectIDFromParams();
    const { data, isLoading, isError } = useNotificationsOverview(projectID);

    if (isError) {
        return <ErrorMessage errorMsg="Error loading notifications" />;
    }

    if (isLoading || !data) {
        return (
            <div className="h-screen w-screen">
                <LoadingScreen />
            </div>
        );
    }

    return (
        <div>
            <PageHeading heading="Notifications" />

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
                <span className="heading">{props.emoji}</span>
            </CardTitle>
            <CardContent>
                <div className="big-heading">
                    {/* 100000 -> 100,000 */}
                    {formatCurrency(props.count, {
                        hideSymbol: true,
                        locale: "en-US",
                    })}
                </div>
            </CardContent>
        </Card>
    );
}
