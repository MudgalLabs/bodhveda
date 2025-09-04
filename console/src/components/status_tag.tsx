import {
    BroadcastStatus,
    NotificationStatus,
} from "@/features/notification/notification_types";
import { statusToString } from "@/lib/utils";
import { Tag, TagVariant } from "netra";

interface StatusTagProps {
    status: NotificationStatus | BroadcastStatus;
}

export function StatusTag(props: StatusTagProps) {
    const { status } = props;

    let variant: TagVariant;

    if (status === "enqueued") {
        variant = "default";
    } else if (status === "completed") {
        variant = "success";
    } else if (status === "failed") {
        variant = "destructive";
    } else if (status === "delivered") {
        variant = "success";
    } else if (status === "muted") {
        variant = "default";
    } else if (status === "quota_exceeded") {
        variant = "destructive";
    } else {
        variant = "default";
    }

    return <Tag variant={variant}>{statusToString(status)}</Tag>;
}
