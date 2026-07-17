import { createFileRoute } from "@tanstack/react-router";

import {
    DEFAULT_RECIPIENT_TAB,
    RECIPIENT_TABS,
    RecipientDetail,
} from "@/features/recipient/detail/recipient_detail";
import { validateViewSearch } from "@/lib/search";

export const Route = createFileRoute("/projects/$id/recipients/$recipientId")({
    // The open tab lives in the URL so a refresh — or a link pasted to someone
    // else — lands on the panel you were actually looking at.
    validateSearch: validateViewSearch(
        "tab",
        RECIPIENT_TABS,
        DEFAULT_RECIPIENT_TAB
    ),
    component: RouteComponent,
});

function RouteComponent() {
    // `recipientId` is the customer-chosen external_id, which may contain
    // URL-hostile characters. The router decodes the param for us — what comes
    // out here is the raw id, and API_ROUTES re-encodes it per request.
    const { recipientId } = Route.useParams();
    const { tab } = Route.useSearch();
    const navigate = Route.useNavigate();

    return (
        <RecipientDetail
            recipientID={recipientId}
            tab={tab}
            // `replace` so switching tabs doesn't pile up history entries the
            // back button has to chew through to reach the recipients list.
            onTabChange={(tab) => navigate({ search: { tab }, replace: true })}
        />
    );
}
