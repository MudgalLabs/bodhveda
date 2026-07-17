import { useMemo } from "react";
import {
    ErrorMessage,
    formatDate,
    IconMegaphone,
    IconTarget,
    IconUsers,
    Loading,
    LoadingScreen,
    PageHeading,
    Tabs,
    TabsContent,
    TabsList,
    TabsTrigger,
    Tooltip,
    useDocumentTitle,
} from "netra";

import { useGetProjectIDFromParams } from "@/features/project/project_hooks";
import { useGetRecipient } from "@/features/recipient/recipient_hooks";
import { RecipientContactsPanel } from "@/features/recipient/detail/recipient_contacts_panel";
import { RecipientNotificationsPanel } from "@/features/recipient/detail/recipient_notifications_panel";
import { RecipientPreferencesPanel } from "@/features/recipient/detail/recipient_preferences_panel";
import { RecipientListItem } from "@/features/recipient/recipient_types";

export const RECIPIENT_TABS = [
    "notifications",
    "preferences",
    "contacts",
] as const;

export type RecipientTab = (typeof RECIPIENT_TABS)[number];

export const DEFAULT_RECIPIENT_TAB: RecipientTab = "notifications";

interface RecipientDetailProps {
    recipientID: string;
    /** The open tab. Owned by the route, which reads it from the URL. */
    tab: RecipientTab;
    onTabChange: (tab: RecipientTab) => void;
}

export function RecipientDetail({
    recipientID,
    tab,
    onTabChange,
}: RecipientDetailProps) {
    useDocumentTitle(`${recipientID} • Recipients • Bodhveda`);

    const projectID = useGetProjectIDFromParams();
    const { data, isLoading, isError, isFetching } = useGetRecipient(
        projectID,
        recipientID
    );

    const recipient = data?.data;

    const content = useMemo(() => {
        if (isError) {
            return (
                <ErrorMessage errorMsg={`Could not load "${recipientID}"`} />
            );
        }

        if (isLoading) {
            return <LoadingScreen />;
        }

        if (!recipient) {
            return <ErrorMessage errorMsg={`Recipient "${recipientID}" not found`} />;
        }

        return (
            <>
                <SummaryStrip recipient={recipient} />

                <Tabs
                    value={tab}
                    onValueChange={(v) => onTabChange(v as RecipientTab)}
                >
                    <TabsList>
                        <TabsTrigger value="notifications">
                            Notifications
                        </TabsTrigger>
                        <TabsTrigger value="preferences">
                            Preferences
                        </TabsTrigger>
                        <TabsTrigger value="contacts">Contacts</TabsTrigger>
                    </TabsList>

                    <TabsContent value="notifications" className="pt-4">
                        <RecipientNotificationsPanel
                            projectID={projectID}
                            recipientID={recipientID}
                        />
                    </TabsContent>

                    <TabsContent value="preferences" className="pt-4">
                        <RecipientPreferencesPanel
                            projectID={projectID}
                            recipientID={recipientID}
                        />
                    </TabsContent>

                    <TabsContent value="contacts" className="pt-4">
                        <RecipientContactsPanel
                            projectID={projectID}
                            recipientID={recipientID}
                        />
                    </TabsContent>
                </Tabs>
            </>
        );
    }, [isError, isLoading, recipient, recipientID, projectID, tab, onTabChange]);

    return (
        <div>
            <PageHeading>
                <IconUsers size={18} />
                <h1 className="select-text!">{recipientID}</h1>
                {isFetching && <Loading />}
            </PageHeading>

            {content}
        </div>
    );
}

/**
 * The recipient's at-a-glance facts, as a strip between the heading and the
 * tabs. These were an Overview tab, which meant the page opened on a handful of
 * fields and made you click to reach anything you came here to do.
 */
function SummaryStrip({ recipient }: { recipient: RecipientListItem }) {
    return (
        <div className="border-border-subtle mb-4 flex flex-wrap items-center gap-x-6 gap-y-2 border-b pb-3 text-sm">
            {/* The external id — the string the customer chose. It IS the page
                heading too, but it lives here as a labeled, selectable field so
                a customer can copy it and find this recipient in their own DB. */}
            <Fact label="External ID">
                <span className="select-text!">{recipient.id}</span>
            </Fact>

            <Fact label="Name">
                {recipient.name || (
                    <span className="text-foreground-muted">—</span>
                )}
            </Fact>

            <Fact label="Created">
                {formatDate(new Date(recipient.created_at), { time: true })}
            </Fact>

            <Fact label="Notifications">
                <span className="flex-x gap-x-4!">
                    <Tooltip content="Direct notifications sent to this recipient">
                        <span className="flex-x gap-x-1.5!">
                            <IconTarget size={14} />
                            {recipient.direct_notifications_count} direct
                        </span>
                    </Tooltip>
                    <Tooltip content="Broadcast notifications this recipient received">
                        <span className="flex-x gap-x-1.5!">
                            <IconMegaphone size={14} />
                            {recipient.broadcast_notifications_count} broadcast
                        </span>
                    </Tooltip>
                </span>
            </Fact>
        </div>
    );
}

function Fact({
    label,
    children,
}: {
    label: string;
    children: React.ReactNode;
}) {
    return (
        <div className="flex-x gap-x-2!">
            <span className="text-foreground-muted text-xs">{label}</span>
            {children}
        </div>
    );
}

