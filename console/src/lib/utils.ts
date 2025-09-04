import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

import {
    BroadcastStatus,
    NotificationStatus,
    Target,
} from "@/features/notification/notification_types";

export function cn(...inputs: ClassValue[]) {
    return twMerge(clsx(inputs));
}

export function isProd(): boolean {
    return process.env.NODE_ENV === "production";
}

export function statusToString(status: NotificationStatus | BroadcastStatus) {
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
