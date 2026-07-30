import { Link } from "@tanstack/react-router";

import { ROW_NAV_LINK_CLASS } from "@/features/notification/components/row_nav";
import { BroadcastListItem } from "@/features/notification/notification_types";
import { useGetProjectIDFromParams } from "@/features/project/project_hooks";

// BroadcastRowNavCell renders the invisible full-row link for a broadcast row.
//
// It replaced a "Peek" dialog plus an "Open" link. The dialog showed a subset of
// what the page shows, so the choice between them was a decision with no upside —
// and a modal cannot be linked, which is the whole reason the page exists.
export function BroadcastRowNavCell({
    broadcast,
}: {
    broadcast: BroadcastListItem;
}) {
    const projectID = useGetProjectIDFromParams();

    return (
        <Link
            to="/projects/$id/broadcasts/$broadcastId"
            params={{ id: projectID, broadcastId: String(broadcast.id) }}
            aria-label={`Broadcast ${broadcast.id}`}
            className={ROW_NAV_LINK_CLASS}
        />
    );
}
