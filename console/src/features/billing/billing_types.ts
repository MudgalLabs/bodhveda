export type PlanID = "free" | "pro";

export interface UserSubscription {
    user_id: number;
    plan_id: PlanID;
    current_period_start: string;
    current_period_end: string;
    created_at: string;
    updated_at: string;
}

export type UsageMetric = "notifications";

export function UsageMetricToString(metric: UsageMetric) {
    switch (metric) {
        case "notifications":
            return "Notifications";
        default:
            return metric;
    }
}

export interface UsageAggregate {
    user_id: number;
    metric: UsageMetric;
    used: number;
    limit: number;
}

export interface GetBillingResult {
    subscription: UserSubscription;
    usage: Record<UsageMetric, UsageAggregate>;
}
