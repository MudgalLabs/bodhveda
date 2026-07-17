import {
    Card,
    CardContent,
    CardTitle,
    ErrorMessage,
    formatNumber,
    IconBell,
    IconDashboard,
    IconSend,
    IconTarget,
    Loading,
    LoadingScreen,
    PageHeading,
    useDocumentTitle,
} from "netra";

import { AnalyticsRange, rangeLabel } from "@/features/dashboard/analytics_range";
import { useProjectAnalytics } from "@/features/dashboard/analytics_hooks";
import { DeliveryHealth, MediumSummary } from "@/features/dashboard/components/medium_summary";
import { NotificationsOverTimeChart } from "@/features/dashboard/components/notifications_over_time_chart";
import { RangePicker } from "@/features/dashboard/components/range_picker";
import { TargetsBreakdownChart } from "@/features/dashboard/components/targets_breakdown_chart";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { ReactNode } from "react";

// The Dashboard is a RANGED analytics page (Phase 9.5): every number on it covers
// the selected range, captioned so it is never mistaken for a lifetime total. It
// replaced the old "Home" page's four lifetime scalars, which were sourced by
// fetching every project and .find()ing the current one — there is now a real
// stats endpoint.
export function Dashboard({
    range,
    onRangeChange,
}: {
    range: AnalyticsRange;
    onRangeChange: (next: AnalyticsRange) => void;
}) {
    useDocumentTitle("Dashboard  • Bodhveda");

    const projectID = useGetProjectIDFromParams();
    const { data, isLoading, isFetching, isError } = useProjectAnalytics(
        projectID,
        range
    );

    const analytics = data?.data;

    let content: ReactNode = null;
    if (isError) {
        content = <ErrorMessage errorMsg="Error loading analytics" />;
    } else if (isLoading || !analytics) {
        content = <LoadingScreen />;
    } else {
        content = (
            <div className="flex flex-col gap-6">
                <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
                    <Kpi
                        title="Notifications"
                        icon={<IconBell size={20} />}
                        count={analytics.in_app.total}
                    />
                    <Kpi
                        title="Delivered"
                        icon={<IconTarget size={20} />}
                        count={analytics.in_app.by_status.delivered}
                    />
                    <Kpi
                        title="Muted"
                        icon={<IconBell size={20} />}
                        count={analytics.in_app.by_status.muted}
                    />
                    <Kpi
                        title="Email sent"
                        icon={<IconSend size={20} />}
                        count={analytics.email.attempted}
                    />
                </div>

                <Section title="Notifications over time">
                    <NotificationsOverTimeChart
                        inApp={analytics.in_app}
                        range={range}
                    />
                </Section>

                <MediumSummary
                    inApp={analytics.in_app}
                    email={analytics.email}
                />

                <DeliveryHealth email={analytics.email} />

                <Section title="Top targets by volume">
                    <TargetsBreakdownChart targets={analytics.targets} />
                </Section>
            </div>
        );
    }

    return (
        <div>
            <PageHeading>
                <IconDashboard size={18} />
                <h1>Dashboard</h1>
                {isFetching && <Loading />}
            </PageHeading>

            <div className="flex flex-col gap-4 p-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                    <RangePicker range={range} onChange={onRangeChange} />
                    <span className="text-text-muted text-xs">
                        Showing {rangeLabel(range).toLowerCase()}
                    </span>
                </div>

                {content}
            </div>
        </div>
    );
}

function Section({
    title,
    children,
}: {
    title: string;
    children: ReactNode;
}) {
    return (
        <div className="border-border-subtle bg-surface-1 rounded-md border p-4">
            <h2 className="text-text-primary mb-3 text-sm font-medium">
                {title}
            </h2>
            {children}
        </div>
    );
}

function Kpi({
    title,
    icon,
    count,
}: {
    title: string;
    icon: ReactNode;
    count: number;
}) {
    return (
        <Card>
            <CardTitle className="flex-x justify-between">
                <span className="font-semibold">{title}</span>
                <span className="text-text-muted">{icon}</span>
            </CardTitle>
            <CardContent>
                <div className="big-heading">{formatNumber(count)}</div>
            </CardContent>
        </Card>
    );
}
