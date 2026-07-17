import {
    ChartConfig,
    ChartContainer,
    ChartLegendContent,
    ChartTooltipContent,
    axisDefaults,
    tooltipCursor,
} from "netra";
import { useMemo } from "react";
import {
    Bar,
    BarChart,
    CartesianGrid,
    Legend,
    ResponsiveContainer,
    Tooltip,
    XAxis,
    YAxis,
} from "recharts";

import { AnalyticsRange, enumerateDays, rangeDayBounds } from "@/features/dashboard/analytics_range";
import { AnalyticsInApp } from "@/features/dashboard/analytics_types";
import { CHART_SURFACE, STATUS_COLORS } from "@/features/dashboard/chart_colors";
import { formatDayShort } from "@/features/dashboard/format";

// The five in-app statuses a `notification` row can hold, in a fixed stacking
// order (settled outcomes at the base, in-flight/failed above). All five are
// really written, so charting them implies no data that cannot exist — unlike
// the reserved DELIVERY statuses, which are never shown.
const STATUS_ORDER = [
    "delivered",
    "muted",
    "quota_exceeded",
    "failed",
    "enqueued",
] as const;

const STATUS_LABEL: Record<(typeof STATUS_ORDER)[number], string> = {
    delivered: "Delivered",
    muted: "Muted",
    quota_exceeded: "Quota exceeded",
    failed: "Failed",
    enqueued: "Enqueued",
};

// NotificationsOverTimeChart is the "send volume over a selectable range" chart:
// a stacked bar per day whose height is total volume and whose segments are the
// in-app outcome mix. Two questions, one chart. In-app is the TRUSTWORTHY medium
// (its `delivered`/`read` are real), so it earns the headline chart; email —
// a soft-signalled subset — gets its own panels below.
export function NotificationsOverTimeChart({
    inApp,
    range,
}: {
    inApp: AnalyticsInApp;
    range: AnalyticsRange;
}) {
    const { data, activeStatuses } = useMemo(() => {
        // Gap-fill: the API returns only days with data, but the axis must be
        // continuous across the whole window (empty edge days included).
        const { start, end } = rangeDayBounds(range);
        const byDay = new Map(inApp.series.map((d) => [d.day, d]));
        const data = enumerateDays(start, end).map((day) => {
            const d = byDay.get(day);
            return {
                day,
                delivered: d?.delivered ?? 0,
                muted: d?.muted ?? 0,
                quota_exceeded: d?.quota_exceeded ?? 0,
                failed: d?.failed ?? 0,
                enqueued: d?.enqueued ?? 0,
            };
        });

        // Only stack statuses that actually occurred in range — a permanently
        // empty series is legend noise.
        const activeStatuses = STATUS_ORDER.filter(
            (s) => inApp.by_status[s] > 0
        );
        return { data, activeStatuses };
    }, [inApp, range]);

    const config: ChartConfig = Object.fromEntries(
        activeStatuses.map((s) => [
            s,
            { label: STATUS_LABEL[s], color: STATUS_COLORS[s] },
        ])
    );

    if (inApp.total === 0) {
        return (
            <EmptyChart message="No notifications sent in this range." />
        );
    }

    return (
        <ChartContainer config={config}>
            <ResponsiveContainer width="100%" height={280}>
                <BarChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                    <CartesianGrid
                        vertical={false}
                        stroke="var(--color-border)"
                        strokeOpacity={0.4}
                    />
                    <XAxis
                        dataKey="day"
                        tickFormatter={formatDayShort}
                        minTickGap={24}
                        {...axisDefaults()}
                    />
                    <YAxis allowDecimals={false} width={52} {...axisDefaults()} />
                    <Tooltip
                        cursor={tooltipCursor}
                        content={
                            <ChartTooltipContent labelFormatter={(l) => formatDayShort(String(l))} />
                        }
                    />
                    <Legend content={<ChartLegendContent />} />
                    {activeStatuses.map((s, i) => (
                        <Bar
                            key={s}
                            dataKey={s}
                            stackId="notifications"
                            fill={STATUS_COLORS[s]}
                            stroke={CHART_SURFACE}
                            strokeWidth={1}
                            // Round only the topmost present segment's top corners.
                            radius={
                                i === activeStatuses.length - 1
                                    ? [4, 4, 0, 0]
                                    : [0, 0, 0, 0]
                            }
                            isAnimationActive={false}
                        />
                    ))}
                </BarChart>
            </ResponsiveContainer>
        </ChartContainer>
    );
}

export function EmptyChart({ message }: { message: string }) {
    return (
        <div className="text-text-muted flex h-[280px] items-center justify-center text-sm">
            {message}
        </div>
    );
}
