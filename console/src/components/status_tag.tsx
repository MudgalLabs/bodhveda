import {
    BroadcastStatus,
    DeliveryStatus,
    NotificationStatus,
} from "@/features/notification/notification_types";
import { statusToString } from "@/lib/utils";
import { Tag, TagVariant } from "netra";

interface StatusTagProps {
    status: NotificationStatus | BroadcastStatus | DeliveryStatus;
}

export function StatusTag(props: StatusTagProps) {
    const { status } = props;

    let variant: TagVariant;

    if (status === "completed") {
        variant = "success";
    } else if (status === "delivered") {
        variant = "success";
    } else if (
        status === "failed" ||
        status === "quota_exceeded" ||
        status === "bounced" ||
        status === "complained" ||
        status === "rejected"
    ) {
        variant = "destructive";
    } else {
        // enqueued, muted, no_contact, suppressed, pending, sending, sent →
        // neutral (in-flight or intentionally-not-delivered outcomes).
        variant = "default";
    }

    return <Tag variant={variant}>{statusToString(status)}</Tag>;
}
