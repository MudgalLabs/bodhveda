import { NotificationsOverviewResult } from "@/features/notification/notification_types";

export interface Project {
    id: number;
    name: string;
    /**
     * When true, a send naming a target this project hasn't cataloged is
     * rejected with a 400. Off by default — see the settings page copy.
     */
    strict_targets: boolean;
}

export const SETTINGS_TABS = ["targeting", "email"] as const;

export type SettingsTab = (typeof SETTINGS_TABS)[number];

export const DEFAULT_SETTINGS_TAB: SettingsTab = "targeting";

export interface CreateProjectPayload {
    name: string;
}

export interface UpdateProjectPayload {
    id: number;
    name: string;
    /**
     * Optional on purpose. The API treats an absent `strict_targets` as "leave
     * it alone", so the rename dialog can keep sending only `name` without
     * silently switching the gate off.
     */
    strict_targets?: boolean;
}

export interface ProjectListItem extends Project, NotificationsOverviewResult {
    total_recipients: number;
}
