import { useQuery } from "@tanstack/react-query";

import { API_ROUTES, APIRes, client } from "@/lib/api";
import { GetBillingResult } from "@/features/billing/billing_types";

// Get the billing information of the current user like
// subscrition plan, usage, invoices, etc.
export function useGetMeBilling() {
    return useQuery({
        queryKey: ["useGetMeBilling"],
        queryFn: () => client.get(API_ROUTES.user.billing),
        select: (res) => res.data as APIRes<GetBillingResult>,
    });
}
