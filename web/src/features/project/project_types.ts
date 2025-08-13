import { NotificationsOverviewResult } from "@/features/home/notification_types";

export interface Project {
    id: number;
    name: string;
}

export interface CreateProjectPayload {
    name: string;
}

export interface ProjectListItem extends Project, NotificationsOverviewResult {
    total_recipients: number;
}
