import { Link } from "@tanstack/react-router";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";

interface RecipientLinkProps {
    recipientID: string;
}

/**
 * A recipient id that navigates to their detail page.
 *
 * `recipientID` is the customer-chosen `external_id` — an arbitrary string that
 * may contain URL-hostile characters. It is passed as a route param, NOT
 * interpolated into a path: the router encodes params on the way out and decodes
 * them on the way back in, so the round trip survives.
 */
export function RecipientLink({ recipientID }: RecipientLinkProps) {
    const projectID = useGetProjectIDFromParams();

    return (
        <Link
            to="/projects/$id/recipients/$recipientId"
            params={{ id: projectID, recipientId: recipientID }}
            className="underline underline-offset-2 hover:text-text-primary"
        >
            {recipientID}
        </Link>
    );
}
