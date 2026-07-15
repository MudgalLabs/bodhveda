import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

import {
    BroadcastStatus,
    DeliveryStatus,
    NotificationStatus,
    Target,
} from "@/features/notification/notification_types";

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs));
}

export function isProd(): boolean {
    return process.env.NODE_ENV === "production";
}

export function statusToString(
    status: NotificationStatus | BroadcastStatus | DeliveryStatus
) {
    switch (status) {
        case "enqueued":
            return "Enqueued";
        case "muted":
            return "Muted";
        case "delivered":
            return "Delivered";
        case "quota_exceeded":
            return "Quota Exceeded";
        case "failed":
            return "Failed";
        case "completed":
            return "Completed";
        case "pending":
            return "Pending";
        case "sending":
            return "Sending";
        case "sent":
            return "Sent";
        case "bounced":
            return "Bounced";
        case "complained":
            return "Complained";
        case "no_contact":
            return "No contact";
        case "suppressed":
            return "Suppressed";
        case "rejected":
            return "Rejected";
        default:
            return status;
    }
}

export function targetToString(target: Target): string {
    let str = "";
    if (target.channel.trim()) str += target.channel.trim();
    if (target.topic.trim()) str += ` : ${target.topic.trim()}`;
    if (target.event.trim()) str += ` : ${target.event.trim()}`;
    return str;
}
