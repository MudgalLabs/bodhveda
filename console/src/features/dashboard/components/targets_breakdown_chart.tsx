import {
    ChartConfig,
    ChartContainer,
    ChartTooltipContent,
    axisDefaults,
    tooltipCursor,
} from "netra";
import {
    Bar,
    BarChart,
    CartesianGrid,
    ResponsiveContainer,
    Tooltip,
    XAxis,
    YAxis,
} from "recharts";

import { AnalyticsTargetStat } from "@/features/dashboard/analytics_types";
import { MAGNITUDE_COLOR } from "@/features/dashboard/chart_colors";
import { EmptyChart } from "@/features/dashboard/components/notifications_over_time_chart";

// How many targets to show. The API caps at 20; the chart shows the busiest ~8
// so the bars stay readable (the rest are in the long tail nobody scans).
const SHOWN = 8;

function targetLabel(t: AnalyticsTargetStat): string {
    return `${t.channel}/${t.topic}/${t.event}`;
}

// TargetsBreakdownChart answers "which targets actually fire": a horizontal bar
// per target, length = notification volume, most-active at top. Color encodes
// NOTHING here (length does), so it is a single sequential hue — not the status
// palette. Email activity for each target rides the tooltip rather than a second
// bar, keeping the magnitude read clean.
export function TargetsBreakdownChart({
    targets,
}: {
    targets: AnalyticsTargetStat[];
}) {
    if (targets.length === 0) {
        return <EmptyChart message="No targets fired in this range." />;
    }

    const data = targets.slice(0, SHOWN).map((t) => ({
        ...t,
        label: targetLabel(t),
    }));

    const config: ChartConfig = {
        notifications: { label: "Notifications", color: MAGNITUDE_COLOR },
    };

    // Give each row room; a fixed bar band reads better than squishing many into
    // a short box.
    const height = Math.max(160, data.length * 40 + 24);

    return (
        <ChartContainer config={config}>
            <ResponsiveContainer width="100%" height={height}>
                <BarChart
                    data={data}
                    layout="vertical"
                    margin={{ top: 0, right: 16, bottom: 0, left: 8 }}
                >
                    <CartesianGrid
                        horizontal={false}
                        stroke="var(--color-border)"
                        strokeOpacity={0.4}
                    />
                    <XAxis type="number" allowDecimals={false} {...axisDefaults()} />
                    <YAxis
                        type="category"
                        dataKey="label"
                        width={160}
                        tick={{ fill: "var(--color-foreground-muted)", fontSize: 12 }}
                        tickLine={false}
                        axisLine={false}
                    />
                    <Tooltip
                        cursor={tooltipCursor}
                        content={
                            <ChartTooltipContent
                                formatter={(value, _name, item) => {
                                    const t = item.payload as AnalyticsTargetStat;
                                    return (
                                        <div className="flex flex-col gap-y-0.5">
                                            <span>
                                                {value} notification
                                                {Number(value) === 1 ? "" : "s"}
                                            </span>
                                            {t.email_attempted > 0 && (
                                                <span className="text-text-muted text-xs">
                                                    Email: {t.email_delivered} delivered
                                                    {t.email_bounced > 0 &&
                                                        `, ${t.email_bounced} bounced`}
                                                    {t.email_complained > 0 &&
                                                        `, ${t.email_complained} complained`}
                                                </span>
                                            )}
                                        </div>
                                    );
                                }}
                            />
                        }
                    />
                    <Bar
                        dataKey="notifications"
                        fill={MAGNITUDE_COLOR}
                        radius={[0, 4, 4, 0]}
                        isAnimationActive={false}
                        barSize={20}
                    />
                </BarChart>
            </ResponsiveContainer>
        </ChartContainer>
    );
}
