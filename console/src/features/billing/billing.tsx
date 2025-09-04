import {
    Alert,
    ErrorMessage,
    formatDate,
    formatNumber,
    IconCreditCard,
    IconInfo,
    Loading,
    LoadingScreen,
    PageHeading,
    useDocumentTitle,
} from "netra";

import { useGetMeBilling } from "@/features/billing/billing_hooks";
import { useMemo } from "react";
import {
    UsageMetric,
    UsageMetricToString,
} from "@/features/billing/billing_types";

export function Billing() {
    useDocumentTitle("Billing  • Bodhveda");

    const { data, isLoading, isFetching, isError } = useGetMeBilling();
    const hasPro = data?.data.subscription.plan_id === "pro";

    const content = useMemo(() => {
        if (isError) {
            return (
                <ErrorMessage errorMsg="Error loading billing information" />
            );
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        if (!data) return null;

        return (
            <div className="max-w-2xl mx-auto">
                <h2 className="paragraph font-medium mb-2">Subscription</h2>

                <Alert>
                    <IconInfo />

                    <div className="flex-x items-start justify-between">
                        <div className="space-y-2">
                            <p className="font-semibold">
                                You are currently on the{" "}
                                {hasPro ? "Pro" : "Free"} plan.
                            </p>

                            <div className="text-text-muted space-y-2">
                                <p>
                                    Renewing on{" "}
                                    {formatDate(
                                        new Date(
                                            data.data.subscription.current_period_end
                                        ),
                                        {
                                            time: true,
                                        }
                                    )}{" "}
                                    along with usage quota.
                                </p>
                            </div>
                        </div>
                    </div>
                </Alert>

                <h2 className="paragraph font-medium mt-8 mb-2">Usage</h2>

                {Object.keys(data.data.usage).map((key) => {
                    const usage = data.data.usage[key as UsageMetric];
                    return (
                        <>
                            <Alert>
                                <IconInfo />

                                <div className="flex-x items-start justify-between">
                                    <div className="space-y-2">
                                        <p className="font-semibold">
                                            {UsageMetricToString(usage.metric)}
                                        </p>

                                        <div className="text-text-muted space-y-2">
                                            <p>
                                                {formatNumber(usage.used)} /{" "}
                                                {formatNumber(usage.limit)}
                                            </p>
                                        </div>
                                    </div>
                                </div>
                            </Alert>
                        </>
                    );
                })}
            </div>
        );
    }, [data, hasPro, isError, isLoading]);

    return (
        <>
            <PageHeading>
                <IconCreditCard size={18} />
                <h1>Billing</h1>
                {isFetching && <Loading />}
            </PageHeading>

            <div className="h-4" />

            {content}
        </>
    );
}
